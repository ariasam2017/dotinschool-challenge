<script setup lang="ts">
// CodeMirror 6 Java editor: syntax highlight, line numbers, bracket matching,
// autocomplete (Java API + live templates + same-file symbols), find/replace,
// and a client-side linter for the basics (unbalanced brackets, unterminated
// strings, likely-missing semicolons) — ported from the original hand-rolled
// editor's lint() so behaviour matches.

import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { EditorState, Compartment } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection, rectangularSelection, crosshairCursor } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { java } from '@codemirror/lang-java'
import { bracketMatching, indentOnInput, foldGutter, syntaxHighlighting, defaultHighlightStyle, HighlightStyle } from '@codemirror/language'
import {
  closeBrackets,
  closeBracketsKeymap,
  autocompletion,
  completionKeymap,
  acceptCompletion,
  completionStatus,
  snippetCompletion,
  type CompletionContext,
  type Completion,
} from '@codemirror/autocomplete'
import { searchKeymap, highlightSelectionMatches, search } from '@codemirror/search'
import { linter, lintGutter, type Diagnostic } from '@codemirror/lint'
import { tags as t } from '@lezer/highlight'
import { isDark } from '@/lib/theme'
import type { EditorProblem, EditorCursor } from './editor-types'

const props = defineProps<{ modelValue: string; disabled?: boolean }>()

const emit = defineEmits<{
  'update:modelValue': [string]
  problems: [EditorProblem[]]
  cursor: [EditorCursor]
}>()

function offsetToLineCol(doc: string, offset: number): { line: number; col: number } {
  const before = doc.slice(0, Math.max(0, offset))
  const nl = before.lastIndexOf('\n')
  return { line: before.split('\n').length, col: offset - (nl + 1) + 1 }
}

const host = ref<HTMLDivElement>()
const view = shallowRef<EditorView>()
const themeComp = new Compartment()
const editableComp = new Compartment()

/* ---- linter ported from app.js lint() ---- */
const CONTROL_START = /^(if|for|while|do|else|switch|try|catch|finally|class|interface|enum)\b/
function lintJava(code: string): Diagnostic[] {
  const lines = code.split('\n')
  const issues: Diagnostic[] = []
  const stack: { ch: string; from: number }[] = []
  const pairs: Record<string, string> = { ')': '(', ']': '[', '}': '{' }
  const opens = new Set(['(', '[', '{'])
  let offset = 0

  for (let li = 0; li < lines.length; li++) {
    const line = lines[li]
    let inStr: string | null = null
    for (let ci = 0; ci < line.length; ci++) {
      const ch = line[ci]
      if (inStr) {
        if (ch === '\\') {
          ci++
          continue
        }
        if (ch === inStr) inStr = null
        continue
      }
      if (ch === '"' || ch === "'") {
        inStr = ch
        continue
      }
      if (ch === '/' && line[ci + 1] === '/') break
      if (opens.has(ch)) stack.push({ ch, from: offset + ci })
      else if (ch in pairs) {
        if (!stack.length || stack[stack.length - 1].ch !== pairs[ch]) {
          issues.push({ from: offset + ci, to: offset + ci + 1, severity: 'error', message: `پرانتز/براکت بسته‌شونده «${ch}» بدون بازشدن متناظر` })
        } else stack.pop()
      }
    }
    if (inStr) {
      issues.push({ from: offset + Math.max(0, line.length - 1), to: offset + line.length, severity: 'error', message: 'رشته متنی بسته نشده است' })
    }
    offset += line.length + 1
  }
  for (const s of stack) {
    issues.push({ from: s.from, to: s.from + 1, severity: 'error', message: `«${s.ch}» باز شده اما بسته نشده است` })
  }

  // heuristic: likely-missing semicolon
  offset = 0
  for (let li = 0; li < lines.length; li++) {
    const raw = lines[li]
    const line = raw.replace(/\/\/.*$/, '')
    const trimmed = line.trim()
    const lineEnd = offset + raw.length
    offset = lineEnd + 1
    if (!trimmed || trimmed.startsWith('@') || trimmed.startsWith('//')) continue
    if (CONTROL_START.test(trimmed)) continue
    if (/[{}[\](,;:+\-*/%<>=!&|?]$/.test(trimmed)) continue
    if (
      /^(return|break|continue|throw)\b/.test(trimmed) ||
      /^[A-Za-z_$][\w$<>[\].]*\s+[A-Za-z_$][\w$]*(\s*=\s*.*)?$/.test(trimmed) ||
      /^[A-Za-z_$][\w$]*(\.[A-Za-z_$][\w$]*)*\s*(\(.*\))?\s*(=[^=].*)?$/.test(trimmed)
    ) {
      const col = raw.trimEnd().length
      issues.push({ from: lineEnd - Math.max(1, raw.length - col + 1) + 1, to: lineEnd, severity: 'warning', message: 'احتمالاً «;» در انتهای این خط جا افتاده است' })
    }
  }
  return issues
}

/* ---- autocomplete: curated Java API + templates + same-file identifiers ---- */
const JAVA_TYPES = ['String', 'Integer', 'Long', 'Double', 'Boolean', 'Character', 'Object', 'List', 'ArrayList', 'LinkedList', 'Map', 'HashMap', 'TreeMap', 'LinkedHashMap', 'Set', 'HashSet', 'TreeSet', 'Queue', 'Deque', 'ArrayDeque', 'Stack', 'Scanner', 'Arrays', 'Collections', 'Math', 'System', 'StringBuilder', 'Comparator', 'Iterator', 'Optional', 'Exception', 'RuntimeException']
const JAVA_METHODS: [string, string][] = [['println', 'System.out.println(x)'], ['print', 'System.out.print(x)'], ['printf', 'System.out.printf(fmt, args)'], ['length', 'array.length / string.length()'], ['size', 'collection.size()'], ['add', 'collection.add(e)'], ['get', 'list.get(i) / map.get(k)'], ['put', 'map.put(k, v)'], ['containsKey', 'map.containsKey(k)'], ['keySet', 'map.keySet()'], ['values', 'map.values()'], ['entrySet', 'map.entrySet()'], ['sort', 'Arrays.sort(a)'], ['max', 'Math.max(a, b)'], ['min', 'Math.min(a, b)'], ['abs', 'Math.abs(x)'], ['equals', 'a.equals(b)'], ['compareTo', 'a.compareTo(b)'], ['substring', 's.substring(a, b)'], ['charAt', 's.charAt(i)'], ['indexOf', 's.indexOf(x)'], ['split', 's.split(regex)'], ['trim', 's.trim()'], ['toCharArray', 's.toCharArray()'], ['parseInt', 'Integer.parseInt(s)'], ['append', 'sb.append(x)'], ['nextInt', 'scanner.nextInt()'], ['nextLine', 'scanner.nextLine()']]
/* IntelliJ IDEA "Live Templates": type the abbreviation, press Tab (or Enter)
   and it expands with proper indentation and tab-stops. Template syntax is
   CodeMirror's snippet syntax: `${}` = final cursor / a stop, `${name}` = a
   placeholder holding literal text, `${1}` `${2}` = numbered stops (repeating a
   number links the fields — editing one edits all), `\t` becomes one indent
   unit. Output text matches IntelliJ's Java templates. */
const LIVE_TEMPLATES: { name: string; template: string; detail: string }[] = [
  { name: 'psvm', template: 'public static void main(String[] args) {\n\t${}\n}', detail: 'public static void main(String[] args)' },
  { name: 'main', template: 'public static void main(String[] args) {\n\t${}\n}', detail: 'public static void main(String[] args)' },
  { name: 'sout', template: 'System.out.println(${});', detail: 'System.out.println(...)' },
  { name: 'souf', template: 'System.out.printf(${});', detail: 'System.out.printf(...)' },
  { name: 'serr', template: 'System.err.println(${});', detail: 'System.err.println(...)' },
  { name: 'soutv', template: 'System.out.println("${1:value} = " + ${1});', detail: 'print a value with its name' },
  { name: 'fori', template: 'for (int ${1:i} = 0; ${1} < ${2:n}; ${1}++) {\n\t${}\n}', detail: 'index for loop' },
  { name: 'forr', template: 'for (int ${1:i} = ${2:n} - 1; ${1} >= 0; ${1}--) {\n\t${}\n}', detail: 'reverse for loop' },
  { name: 'iter', template: 'for (${1:var} ${2:item} : ${3:iterable}) {\n\t${}\n}', detail: 'enhanced for (for-each)' },
  { name: 'itar', template: 'for (int ${1:i} = 0; ${1} < ${2:array}.length; ${1}++) {\n\t${3:int} ${4:x} = ${2}[${1}];\n\t${}\n}', detail: 'iterate over an array' },
  { name: 'iff', template: 'if (${}) {\n\t${}\n}', detail: 'if statement' },
  { name: 'ifn', template: 'if (${1:x} == null) {\n\t${}\n}', detail: 'if (x == null)' },
  { name: 'inn', template: 'if (${1:x} != null) {\n\t${}\n}', detail: 'if (x != null)' },
  { name: 'ifelse', template: 'if (${}) {\n\t${}\n} else {\n\t\n}', detail: 'if / else' },
  { name: 'whilen', template: 'while (${}) {\n\t${}\n}', detail: 'while loop' },
  { name: 'trycatch', template: 'try {\n\t${}\n} catch (${1:Exception} ${2:e}) {\n\t${3:e.printStackTrace();}\n}', detail: 'try / catch' },
  { name: 'try', template: 'try {\n\t${}\n} catch (${1:Exception} ${2:e}) {\n\t${3:e.printStackTrace();}\n}', detail: 'try / catch' },
  { name: 'thr', template: 'throw new ${1:RuntimeException}(${2});', detail: 'throw an exception' },
  { name: 'psf', template: 'public static final ${}', detail: 'public static final' },
  { name: 'psfi', template: 'public static final int ${1:NAME} = ${2:0};', detail: 'public static final int' },
  { name: 'psfs', template: 'public static final String ${1:NAME} = "${2}";', detail: 'public static final String' },
  { name: 'prsf', template: 'private static final ${}', detail: 'private static final' },
  { name: 'St', template: 'String', detail: 'String' },
]

function javaCompletions(ctx: CompletionContext) {
  const word = ctx.matchBefore(/[\w$]+/)
  if (!word || (word.from === word.to && !ctx.explicit)) return null
  const prefix = ctx.state.sliceDoc(word.from, word.to)
  const lower = prefix.toLowerCase()
  const doc = ctx.state.doc.toString()

  const ids = new Set<string>()
  for (const m of doc.matchAll(/[A-Za-z_$][A-Za-z0-9_$]*/g)) ids.add(m[0])
  ids.delete(prefix)

  const templates: Completion[] = LIVE_TEMPLATES.filter((tpl) =>
    tpl.name.toLowerCase().startsWith(lower),
  ).map((tpl) =>
    snippetCompletion(tpl.template, {
      label: tpl.name,
      type: 'keyword',
      detail: 'Live Template — ' + tpl.detail,
      boost: 99, // live templates rank first, like IntelliJ
    }),
  )

  const options: Completion[] = [
    ...templates,
    ...JAVA_TYPES.map((label) => ({ label, type: 'class', detail: 'کلاس / نوع' })),
    ...JAVA_METHODS.map(([label, detail]) => ({ label, type: 'function', detail, apply: label + '()' })),
    ...[...ids].map((label) => ({ label, type: 'variable', detail: 'نماد در همین فایل' })),
  ]
  return { from: word.from, options, validFor: /^[\w$]*$/ }
}

const highlightDark = HighlightStyle.define([
  { tag: t.keyword, color: '#c084fc' },
  { tag: [t.string, t.special(t.string)], color: '#86efac' },
  { tag: [t.number, t.bool, t.null], color: '#fdba74' },
  { tag: [t.function(t.variableName), t.function(t.propertyName)], color: '#60a5fa' },
  { tag: [t.typeName, t.className], color: '#5eead4' },
  { tag: t.comment, color: '#64748b', fontStyle: 'italic' },
  { tag: t.operator, color: '#94a3b8' },
])

function makeThemeExt() {
  const base = EditorView.theme(
    {
      '&': { height: '100%', fontSize: '13.5px', backgroundColor: 'var(--code-bg)', color: 'var(--code-text)' },
      '.cm-scroller': { fontFamily: 'var(--font-mono)', lineHeight: '1.65', direction: 'ltr' },
      '.cm-gutters': { backgroundColor: 'var(--code-bg)', color: 'var(--text-faint)', border: 'none' },
      '.cm-activeLineGutter, .cm-activeLine': { backgroundColor: 'rgba(148,163,184,0.08)' },
      '.cm-content': { caretColor: 'var(--primary)' },
      '&.cm-focused .cm-cursor': { borderLeftColor: 'var(--primary)' },
      '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': { backgroundColor: 'rgba(99,102,241,0.28)' },
      '.cm-panels': { backgroundColor: 'var(--surface)', color: 'var(--text)', direction: 'rtl' },
      '.cm-searchMatch': { backgroundColor: 'rgba(245,158,11,0.32)' },
      '.cm-tooltip': { backgroundColor: 'var(--surface)', border: '1px solid var(--border)', borderRadius: '10px', color: 'var(--text)' },
    },
    { dark: isDark.value },
  )
  return [base, syntaxHighlighting(isDark.value ? highlightDark : defaultHighlightStyle, { fallback: true })]
}

onMounted(() => {
  const state = EditorState.create({
    doc: props.modelValue,
    extensions: [
      lineNumbers(),
      highlightActiveLineGutter(),
      highlightActiveLine(),
      history(),
      foldGutter(),
      drawSelection(),
      rectangularSelection(),
      crosshairCursor(),
      indentOnInput(),
      bracketMatching(),
      closeBrackets(),
      autocompletion({
        override: [javaCompletions],
        activateOnTyping: true,
        selectOnOpen: true,
        defaultKeymap: false, // we bind the completion keys ourselves, below
        icons: true,
      }),
      highlightSelectionMatches(),
      search({ top: true }),
      java(),
      lintGutter(),
      linter(
        (v) => {
          const doc = v.state.doc.toString()
          const diags = lintJava(doc)
          emit(
            'problems',
            diags.map((d) => {
              const lc = offsetToLineCol(doc, d.from)
              return {
                severity: d.severity === 'error' ? 'error' : 'warning',
                message: d.message,
                line: lc.line,
                col: lc.col,
              } as EditorProblem
            }),
          )
          return diags
        },
        { delay: 350 },
      ),
      keymap.of([
        // IntelliJ-style: Tab accepts the highlighted completion / live template
        // (its snippet then expands with indentation and tab-stops). When no
        // completion popup is open, Tab indents as usual.
        {
          key: 'Tab',
          run: (view) => (completionStatus(view.state) === 'active' ? acceptCompletion(view) : false),
        },
        ...closeBracketsKeymap,
        ...completionKeymap, // ArrowUp/Down, Enter-to-accept, Esc, Ctrl-Space
        ...defaultKeymap,
        ...historyKeymap,
        ...searchKeymap,
        indentWithTab,
      ]),
      themeComp.of(makeThemeExt()),
      editableComp.of(EditorView.editable.of(!props.disabled)),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) emit('update:modelValue', u.state.doc.toString())
        if (u.docChanged || u.selectionSet) {
          const head = u.state.selection.main.head
          emit('cursor', offsetToLineCol(u.state.doc.toString(), head))
        }
      }),
    ],
  })
  view.value = new EditorView({ state, parent: host.value! })
})

onBeforeUnmount(() => view.value?.destroy())

watch(
  () => props.modelValue,
  (v) => {
    const cur = view.value?.state.doc.toString()
    if (view.value && v !== cur) {
      view.value.dispatch({ changes: { from: 0, to: cur!.length, insert: v } })
    }
  },
)
watch(
  () => props.disabled,
  (d) => view.value?.dispatch({ effects: editableComp.reconfigure(EditorView.editable.of(!d)) }),
)
watch(isDark, () => view.value?.dispatch({ effects: themeComp.reconfigure(makeThemeExt()) }))

defineExpose({
  focus: () => view.value?.focus(),
  getValue: () => view.value?.state.doc.toString() ?? '',
})
</script>

<template>
  <div ref="host" class="java-editor" :class="{ 'is-disabled': disabled }" />
</template>

<style scoped>
.java-editor {
  height: 100%;
  min-height: 320px;
  border-radius: var(--radius);
  overflow: hidden;
  border: 1px solid var(--border);
  background: var(--code-bg);
}
.java-editor :deep(.cm-editor) {
  height: 100%;
}
.java-editor.is-disabled {
  opacity: 0.65;
}
</style>
