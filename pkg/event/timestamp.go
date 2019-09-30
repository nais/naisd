package deployment

import (
	"time"
)

// GetTimestampAsTime returns the Event's timestamp as standard library format.
func (m *Event) GetTimestampAsTime() time.Time {
	return time.Unix(m.GetTimestamp().GetSeconds(), int64(m.GetTimestamp().GetNanos()))
}
