/** The first-ever L&F report number used as an optional immortal test item. */
export const SACRED_LOST_FOUND_REPORT_NUMBER = 'LF-0001';

/**
 * Sacred LF protection is opt-in via system_settings.preserve_sacred_lostfound.
 * Default false so reusable deployments are not stuck with LF-0001 immortal.
 */
export function isSacredLostFoundReport(
  reportNumber: string,
  preserveSacred = false
): boolean {
  if (!preserveSacred) return false;
  return reportNumber.trim().toUpperCase() === SACRED_LOST_FOUND_REPORT_NUMBER;
}
