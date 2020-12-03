package deck

import "fmt"

// The "clips count" command returns the number of clips on the current timeline.
func clipsCount(d *Deck) string {
	return fmt.Sprintf("214 clips count:\r\nclip count: %v\r\n", len(d.Clips))
}

func remoteInfo(d *Deck) string {
	return fmt.Sprintf("210 remote info:\r\nenabled: %v\r\noverride: %v\r\n", d.Remote.Enabled, d.Remote.Override)
}
