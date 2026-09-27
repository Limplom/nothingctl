package notifclip

import (
	"strings"
	"testing"
)

const sampleDumpsysNotification = `Current Notification Manager state:
  Notification List:
    NotificationRecord(0x0a1b2c3d: pkg=com.whatsapp user=UserHandle{0} id=1 tag=null importance=4 key=0|com.whatsapp|1|null|10123: Notification(channel=msg))
      uid=10123 userId=0
      extras={
        android.title=String (Alice)
        android.text=SpannableString (Hi there, how are you?)
        android.subText=null
      }
    NotificationRecord(0x0b2c3d4e: pkg=com.android.systemui user=UserHandle{0} id=2 tag=null importance=2 key=0|com.android.systemui|2|null|10050: Notification(channel=usb))
      extras={
        android.title=null
        android.text=null
      }
    NotificationRecord(0x0c3d4e5f: pkg=com.example.plain user=UserHandle{0} id=3)
      extras={
        android.title=Plain title
      }
`

func TestParseNotifications(t *testing.T) {
	crlf := strings.ReplaceAll(sampleDumpsysNotification, "\n", "\r\n")
	for name, in := range map[string]string{"lf": sampleDumpsysNotification, "crlf": crlf} {
		t.Run(name, func(t *testing.T) {
			got := parseNotifications(in)
			want := []notification{
				{"com.whatsapp", "Alice", "Hi there, how are you?"},
				{"com.android.systemui", "(no title)", "(no text)"},
				{"com.example.plain", "Plain title", "(no text)"},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d notifications, want %d: %+v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("notification %d = %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
}

func TestParseNotificationsEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"no records", "Current Notification Manager state:\n  nothing here\n", 0},
		{"record without pkg", "NotificationRecord(0x1: id=1)\n  android.title=String (x)\n", 0},
	}
	for _, c := range cases {
		if got := parseNotifications(c.in); len(got) != c.want {
			t.Errorf("%s: got %d notifications, want %d: %+v", c.name, len(got), c.want, got)
		}
	}
}

func TestSplitOnBoundary(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"no boundary", "abc", []string{"abc"}},
		{"leading boundary", "XaXb", []string{"Xa", "Xb"}},
		{"prefix before boundary", "preXaXb", []string{"pre", "Xa", "Xb"}},
		{"adjacent boundaries", "XX", []string{"X", "X"}},
	}
	for _, c := range cases {
		got := splitOnBoundary(c.text, "X")
		if len(got) != len(c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %q, want %q", c.name, got, c.want)
				break
			}
		}
	}
}

func TestDecodeParcelUTF16(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// status=0, length=5, "hello" as UTF-16LE packed into LE 32-bit words.
		{"hello", "Result: Parcel(00000000 00000005 00650068 006c006c 0000006f)", "hello"},
		{"no match", "service: not found", ""},
		{"too short", "Result: Parcel(00000000)", ""},
		{"empty payload", "Result: Parcel(00000000 00000000)", ""},
	}
	for _, c := range cases {
		if got := decodeParcelUTF16(c.in); got != c.want {
			t.Errorf("%s: decodeParcelUTF16 = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestDecodeParcelUTF16RealFormat documents a suspected mismatch: real
// `service call` output always carries an ASCII preview column
// (e.g. "Result: Parcel(00000000 00000000   '........')") and longer parcels
// are printed multi-line with "0x00000000:" offsets. parcelRe only accepts
// hex+whitespace up to ")", so neither form matches. The payload of
// `service call clipboard 2` is also a ClipData parcelable rather than a bare
// String16, so the right fix is unclear without a device sample.
func TestDecodeParcelUTF16RealFormat(t *testing.T) {
	t.Skip("parcelRe does not accept the ASCII preview column of real `service call` output; needs device-verified fix")
	in := "Result: Parcel(00000000 00000005 00650068 006c006c 0000006f '....h.e.l.l.o...')"
	if got := decodeParcelUTF16(in); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}
