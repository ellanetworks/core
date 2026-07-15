// Date formatting is timezone-dependent, so a fixed zone is the only way these
// assertions mean the same thing on a developer machine and in CI.
process.env.TZ = "UTC";
