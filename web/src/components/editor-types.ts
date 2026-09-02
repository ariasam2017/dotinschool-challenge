// Shared types for JavaEditor.vue (a <script setup> block cannot `export`).

export interface EditorProblem {
  severity: 'error' | 'warning'
  message: string
  line: number
  col: number
}

export interface EditorCursor {
  line: number
  col: number
}
