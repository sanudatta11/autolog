// Time intervals
export const LOGS_POLL_INTERVAL_MS = 10 * 1000; // 10 seconds

// Progress bar
export const UPLOAD_PROGRESS_BAR_HEIGHT = 3; // px

// PDF generation
export const PDF_SUMMARY_WRAP_WIDTH = 170;
export const PDF_ROOT_CAUSE_WRAP_WIDTH = 170;
export const PDF_RECOMMENDATION_WRAP_WIDTH = 160;
export const PDF_TABLE_COLUMN_WIDTHS = {
  pattern: 22,
  count: 10,
  severity: 18,
  first: 24,
  last: 24,
  rootCause: 38,
  impact: 38,
  fix: 38,
  related: 22,
};

// Add more constants as needed and group them by feature/module. 