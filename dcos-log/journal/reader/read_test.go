package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/journal"
)

func getUniqueString() string {
	return fmt.Sprintf("%d", time.Now().Nanosecond())
}

func sendEntry(msg, key, value string) error {
	return journal.Send(msg, journal.PriInfo, map[string]string{key: value})
}

func sendUniqueEntry() (string, error) {
	testMessage := getUniqueString()
	err := sendEntry(testMessage, "CUSTOM_FIELD", testMessage)
	return testMessage, err
}

func TestJournalReaderAllLogs(t *testing.T) {
	testMessage, err := sendUniqueEntry()
	if err != nil {
		t.Fatal(err)
	}

	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeText,
	}

	r, err := NewReader(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	var foundEntry bool
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), testMessage) {
			t.Logf("Found! %s", scanner.Text())
			foundEntry = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if !foundEntry {
		t.Fatal("Entry not found :(")
	}
}

func TestJournalLimit(t *testing.T) {
	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeText,
		Limit:       10,
	}

	r, err := NewReader(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(r)
	var counter int
	for scanner.Scan() {
		counter++
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if counter != 10 {
		t.Fatalf("Expecting 10 lines, got: %d", counter)
	}
}

func TestJournalFind(t *testing.T) {
	first, err := sendUniqueEntry()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		_, err := sendUniqueEntry()
		if err != nil {
			t.Fatal(err)
		}
	}
	// wait for journal entries to commit
	time.Sleep(time.Millisecond * 100)

	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeText,
		Matches: []Match{
			{
				Field: "CUSTOM_FIELD",
				Value: first,
			},
		},
	}

	r, err := NewReader(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(r)
	var size int
	for scanner.Scan() {
		if !strings.Contains(scanner.Text(), first) {
			t.Fatalf("Was looking for %s. Got %s", first, scanner.Text())
		}
		size++
	}
	if size != 1 {
		t.Fatalf("Must have only 1 entry. Got %d", size)
	}
}

func TestJournalSkipForward(t *testing.T) {
	uniq := getUniqueString()
	err := sendEntry(uniq, "CUSTOM_FIELD", uniq)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		sendEntry(fmt.Sprintf("index-%d", i), "CUSTOM_FIELD", uniq)
	}
	// wait for journal entries to commit
	time.Sleep(time.Millisecond * 100)

	journalConfig := JournalReaderConfig{
		ContentType: ContentTypeJSON,
		SkipNext:    2,
		Matches: []Match{
			{
				Field: "CUSTOM_FIELD",
				Value: uniq,
			},
		},
	}

	r, err := NewReader(journalConfig)
	if err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(r)
	var size int
	type response struct {
		Fields map[string]string
	}
	for scanner.Scan() {
		r := response{}
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		value, ok := r.Fields["MESSAGE"]
		if !ok {
			t.Fatalf("Field MESSAGE not found. Got: %v", r)
		}
		expectedString := fmt.Sprintf("index-%d", size+1)
		if value != expectedString {
			t.Fatalf("Expected: %s. Got %s", expectedString, value)
		}
		size++
	}
	if size != 3 {
		t.Fatalf("Must have 3 entries. Got %d", size)
	}
}
