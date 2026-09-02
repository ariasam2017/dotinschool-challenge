import type { MessageTree } from './index'

// English — optional stub, kept key-for-key in sync with fa.ts. Incomplete keys
// fall back to Persian at runtime.
export const en: MessageTree = {
  app: {
    userTitle: 'Coding Challenge',
    adminTitle: 'Admin Panel',
  },
  nav: {
    overview: 'Event Overview',
    schedule: 'Scheduling',
    challenge: 'Challenge Editor',
    participants: 'Participants',
    ai: 'AI Review',
    assistant: 'Code Assistant',
    errors: 'User Errors',
    logout: 'Log out',
  },
  auth: {
    register: 'Sign up',
    login: 'Log in',
    firstName: 'First name',
    lastName: 'Last name',
    phone: 'Mobile number',
    otp: 'SMS verification code',
    sendOtp: 'Send SMS code',
    resendIn: 'Resend ({sec})',
    submitRegister: 'Sign up & view challenge',
    submitLogin: 'Log in',
    rememberNote: 'This device is remembered — next time you sign in without a code.',
  },
  event: {
    waiting: 'The event is scheduled and has not started yet',
    paused: 'The event is temporarily paused by the admin',
    ended: 'The event has ended',
    remaining: 'Time remaining',
    untilStart: 'until auto-start',
    untilEnd: 'until the event ends',
  },
  challenge: {
    start: 'Start solving',
    submit: 'Submit code',
    resubmit: 'Submit again',
    retry: 'Try again',
    sampleIn: 'Sample input',
    sampleOut: 'Sample output',
    scoreByCorrectness: 'Scored by output correctness',
    scoreBySpeed: 'Scored by execution speed — target: {ms} ms',
  },
  common: {
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    edit: 'Edit',
    refresh: 'Refresh',
    close: 'Close',
    loading: 'Loading…',
  },
}
