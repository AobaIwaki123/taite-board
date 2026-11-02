/**
 * Core Idol Schedule Domain Logic & Types (Framework-Agnostic)
 * Reusable across Web, Mobile Native (iOS / Android), CLI, and Portal Hub.
 */

// Domain Types
export type {
  ScheduleData,
  Session,
  TicketInfo,
  TicketPrice,
  Venue,
  LivePerformance,
  BenefitEvent,
  PhotoRule,
  Promotion,
  SessionImage,
  SourceRef,
  SessionStatus,
} from '../types/schedule';

// Date, Time & Countdown Utilities
export {
  parseDeadlineDate,
  getDeadlineInfo,
  getFreshnessInfo,
  isSessionPast,
  resolveNextSession,
  getSessionProgress,
  getTodayStr,
} from '../utils/date';

// Expense & Ticket Accounting Utilities
export {
  CHEKI_SIGNED_PRICE,
  CHEKI_REGULAR_PRICE,
  DRINK_FEE_OPTIONS,
  defaultSessionExpense,
  calculateExpenseTotal,
  parseTicketFee,
  isIncrementalFee,
  calculateMonthExpenseSummary,
} from '../utils/expense';
export type {
  DrinkFeeOption,
  SessionExpense,
  WeekExpenseSummary,
  MonthExpenseSummary,
} from '../utils/expense';

// Attendance & Delay Utilities
export {
  getAttendedSessionIds,
  toggleAttendedSessionId,
  isSessionAttended,
} from '../utils/attendance';

export {
  getSessionDelayOffset,
  setSessionDelayOffset,
} from '../utils/delay';

// Multi-Tenant / Group Configuration
export {
  resolveCurrentGroup,
  REGISTERED_GROUPS,
  DEFAULT_GROUP_ID,
} from '../config/group';
export type {
  GroupConfig,
  GroupTheme,
} from '../config/group';
