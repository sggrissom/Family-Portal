# When an optional dependency is down

Three subsystems talk to something outside the process: face analysis (the dlib
daemon over a unix socket), AI import (Gemini over the internet), and push
notifications (APNs). All three are optional, all three will be unavailable
sometimes, and none of them may take primary user data with them.

"Primary user data" means the record a person created: the account, the person,
the measurement, the milestone, the photo and its file, the chat message. Those
are what a family would notice losing. Everything the three subsystems produce —
a suggested face tag, an AI-drafted set of records, a lock-screen notification —
is derived, regenerable, or merely convenient.

The rule, in one line: **an optional dependency failing may cost its own output
and nothing else.**

## Face analysis

| condition | behavior |
| --- | --- |
| socket missing at startup | `InitializeAnalysisWorker` logs and returns. No worker exists; `QueuePhotoAnalysis` becomes a no-op. Uploads are unaffected. |
| daemon dies while running | Each job fails its socket call, the photo's `AnalysisStatus` is set to `3` (failed), and the loop continues to the next job. |
| queue full | The job is dropped with a log line. The photo keeps every other property. |
| local build | `photo_analysis_worker_stub.go` — every entry point is a no-op and `cfg.EnableFaceTagging` is false. |
| shutdown | Stopped without draining. See `StopAnalysisWorker`. |

A photo that misses analysis keeps its pixels, its date, its caption, and every
tag a person applied by hand. Nothing about it is broken; it just has no
suggestions. An admin can requeue the backlog with "reanalyze all photos".

The upload path never waits on analysis: `QueuePhotoAnalysis` is called *after*
the photo worker has already marked the photo complete, and it neither blocks
nor returns an error. `make e2e` covers this — the end-to-end run has no face
daemon, and a photo upload still has to finish.

## AI import

| condition | behavior |
| --- | --- |
| `GEMINI_API_KEY` unset | `ProcessAIImport` returns "AI not configured" in `resp.Error`. No records are written. |
| Gemini unreachable or slow | The 60-second client timeout fires and the transport error is returned in `resp.Error`, with the request URL stripped so no endpoint or credential reaches the browser. |
| Gemini returns nonsense | Parsing fails and the error is returned. Nothing is written. |

AI import is a *drafting* step: it converts text into records the user then
reviews and imports. A failure costs the draft, and the source text is still in
the box the user pasted it into. No other request path calls Gemini, so an
outage there cannot affect a login, a photo, or a measurement.

## Push notifications

| condition | behavior |
| --- | --- |
| APNs not configured | `InitializePushWorker` logs and returns without creating a worker. `QueuePushNotification` returns an error, which `SendMessage` logs and ignores. |
| APNs unreachable | The send fails and is logged. The chat message is already committed. |
| device token rejected | The registration is deactivated. Nothing else changes. |
| queue full | `QueuePushNotification` returns an error; again logged and ignored. |

The ordering is the point: chat messages are written and broadcast over
WebSocket *before* the notification is queued. A push failure means someone's
phone did not buzz, not that the message was lost — it is in the room, and the
web client already has it.

## What this rules out

- No handler may block on an optional dependency's response before committing
  the user's own write.
- No optional dependency may return an error that fails an otherwise-successful
  request.
- No queue may be unbounded. All four are `chan` with a fixed capacity and a
  non-blocking `select` on send, so a stalled consumer causes dropped derived
  work rather than a stalled request.

## Shutdown

`ShutdownWorkers` (backend/shutdown.go) drains photo processing, mail, and push,
and stops face analysis without draining. The ordering and the reasoning for
each are in that file.
