# When an optional dependency is down

Four subsystems talk to something outside the process: face analysis (the
dlib daemon over a unix socket), vision analysis (the CLIP/MiniLM daemon over a
unix socket), video processing (ffmpeg and ffprobe, exec'd), and push
notifications (APNs). Both are optional,
both will be unavailable sometimes, and neither may take primary user data with
them.

"Primary user data" means the record a person created: the account, the person,
the measurement, the milestone, the photo and its file, the chat message. Those
are what a family would notice losing. Everything the two subsystems produce —
a suggested face tag, a lock-screen notification —
is derived, regenerable, or merely convenient.

The rule, in one line: **an optional dependency failing may cost its own output
and nothing else.**

## Face analysis

| condition | behavior |
| --- | --- |
| socket missing at startup | `InitializeAnalysisWorker` logs and returns. No worker exists; `QueuePhotoAnalysis` becomes a no-op. Uploads are unaffected. |
| daemon dies while running | Each job fails its socket call, the photo's `AnalysisStatus` is set to `3` (failed), and the loop continues to the next job. |
| queue full | The job moves to an in-memory backlog the worker drains between live jobs. Anything lost on restart is re-queued by the startup sweep. |
| local build | `cfg.EnableFaceTagging` is false, so the worker is never created and every entry point is a no-op. |
| shutdown | Stopped without draining. See `StopAnalysisWorker`. |

A photo that misses analysis keeps its pixels, its date, its caption, and every
tag a person applied by hand. Nothing about it is broken; it just has no
suggestions. An admin can requeue the backlog with "reanalyze all photos".

The upload path never waits on analysis: `QueuePhotoAnalysis` is called *after*
the photo worker has already marked the photo complete, and it neither blocks
nor returns an error. `make e2e` covers this — the end-to-end run has no face
daemon, and a photo upload still has to finish.

## Vision analysis

| condition | behavior |
| --- | --- |
| not configured (local builds) | `cfg.VisionAnalysisSocket` is empty, so no worker exists and `QueuePhotoEmbedding` is a no-op. |
| daemon down, at startup or later | The embedding job goes back on the front of the backlog and the worker pauses, 5 seconds doubling to 5 minutes, then tries again. Nothing is marked failed. |
| daemon rejects one image | Logged; the photo stays without an embedding until the next startup sweep. |
| backlog | One entry per photo id at most, so it is bounded by the size of the library. |
| shutdown | Stopped without draining; the startup sweep requeues whatever is missing. |

A photo without an embedding is simply absent from features that rank by
embedding. Uploads and every other page are unaffected.

## Video

Video differs from the others in one way: a video's file *is* primary user
data. So the rule here is that nothing the dependency does may lose an
accepted original, and nothing may be accepted that cannot be kept.

| condition | behavior |
| --- | --- |
| ffmpeg or ffprobe missing at startup | `InitializeVideoWorker` logs and returns. `GetVideoSupport` reports video unavailable and `BeginVideoUpload` refuses with a clear message, so no new upload is accepted that could not be processed. Existing videos keep playing. |
| `cfg.EnableVideo` off, or media storage unavailable | The same refusal. Existing video rows render as their poster or a placeholder. |
| `MEDIA_S3_*` incomplete | Media storage is unavailable (never a silent fallback to local disk), with the missing settings in admin health. |
| bucket unreachable | Startup logs it and carries on. Uploads, playback links and worker jobs fail or retry as for any storage error; nothing already stored is lost, and the janitor keeps retrying deletes. |
| family not enabled | `BeginVideoUpload` refuses with `ErrVideoDisabled`. Video is opt-in per family. |
| disk short of the reservation a job needs | The job goes back on the front of the backlog and the worker pauses, 5 seconds doubling to 5 minutes. Nothing is marked failed and `MinFreeDiskBytes` is never crossed. |
| storage read or write fails mid-job | Same retry. The row stays processing. |
| ffmpeg cannot remux or make a poster | The row is marked failed with a reason and **keeps its original**, which stays downloadable. An admin can retry with "retry failed videos" once the cause is fixed. |
| the file is not a usable video (unreadable, too long, unsupported codec) | Rejected: the row is marked failed with a reason the uploader sees, the original is deleted, and its storage released. |
| restart or shutdown mid-job | ffmpeg is cancelled and scratch files removed; the row stays processing and the startup sweep requeues it. |
| a delete of a stored object fails | A `MediaDeletion` row, written with the delete that dropped the reference, is retried by the media janitor every ten minutes until the store confirms. |

Rejections are the uploader's file, not a fault, so they are counted in admin
health without making the system unhealthy. Processing failures, videos stuck
processing for six hours, deletes failing repeatedly, and a family with video
on while processing is unavailable all do.

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
- No queue may be unbounded. The four `chan` queues have a fixed capacity and a
  non-blocking `select` on send, so a stalled consumer causes dropped derived
  work rather than a stalled request. The features and vision backlogs hold
  photo ids, deduplicated, so they can never outgrow the library, and adding
  to them never blocks.

## Shutdown

`ShutdownWorkers` (backend/shutdown.go) drains photo processing, mail, and push,
and stops face analysis without draining. The ordering and the reasoning for
each are in that file.
