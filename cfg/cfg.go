package cfg

const Backport = 12999

// Port is the port the application listens on, in every build. Both server
// entry points read it from here rather than declaring their own, because it
// is no longer only their business: the admin panel's backup check sends the
// application a request over loopback and has to know where "itself" is, and a
// port that disagreed with the listener would fail the check for the one
// reason that has nothing to do with backups.
//
// backupctl fetches the snapshot from this same port (`snapshot_url` in
// shared/backup.conf), so it is also what that file has to name.
const Port = 8666
