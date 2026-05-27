package bundle

import "time"

// zeroTime is used as the mtime for every tar entry so the resulting bytes
// (and therefore the bundle hash) depend only on the directory tree, not
// on filesystem metadata.
var zeroTime = time.Unix(0, 0).UTC()
