# Location sharing

Show where family members are on a map, with optional notifications for arriving
at or leaving known places like home, school, or work.

This one is different from the other future features in ways that matter:
- It needs the mobile app to report position in the background, so it is
  primarily mobile app work, not backend work.
- It generates continuous writes rather than occasional ones, so retention has
  to be decided up front. Keeping current position plus a short trail is very
  different from keeping history forever.
- It is the most sensitive data the app would ever hold, and the kids being
  tracked are not the ones who chose to install it. Per-person visibility
  controls and a clear, obvious indicator that sharing is on are requirements,
  not polish.

Given the mobile dependency and the privacy weight, this should come last.
