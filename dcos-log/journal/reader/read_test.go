package reader

import (
	"testing"
	"bytes"
	"io"
	"strings"
	"time"
	"fmt"

	"github.com/coreos/go-systemd/journal"
)

func sendEntry(msg, key, value string) error {
	return journal.Send(msg, journal.PriInfo, map[string]string{key: value})
}

func sendUniqueEntry() (string, error) {
	testMessage := fmt.Sprintf("%d", time.Now().UnixNano())
	err := sendEntry(testMessage, "JournalTEST", testMessage)
	return testMessage, err
}

func readJournal(cfg JournalReaderConfig) ([]string, error) {
	buf := new(bytes.Buffer)
	j, err := NewReader(cfg)
	if err != nil {
		return []string{}, err
	}
	defer j.Journal.Close()

	_, err = io.Copy(buf, j)
	if err != nil {
		return []string{}, err
	}

	entries := strings.Split(buf.String(), "\n")
	if len(entries) == 0 {
		return []string{}, fmt.Errorf("Received empty logs: %s", entries)
	}
	return entries, nil
}

func TestJournalReaderAllLogs(t *testing.T) {
	testMessage, err := sendUniqueEntry()
	if err != nil {
		t.Fatal(err)
	}

	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeText,
	}

	entries, err := readJournal(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	var foundEntry bool
	for _, entry := range entries[len(entries)-100:] {
		if strings.Contains(entry, testMessage) {
			t.Logf("Found!: %s", entry)
			foundEntry = true
		}
	}

	if !foundEntry {
		t.Fatal("Could not find message")
	}
}

func TestJournalLimit(t *testing.T) {
	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeText,
		Limit: 10,
	}

	entries, err := readJournal(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	// line 11 is empty line
	if len(entries) != 11 {
		t.Fatalf("Must be 10 entries, got %d", len(entries))
	}
	if entries[10] != "" {
		t.Fatalf("Last line must be empty. Got %s", entries[10])
	}
}