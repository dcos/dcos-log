package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"bytes"
	"context"
	"io"

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

	r, err := NewReader(FormatText{})
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
	r, err := NewReader(FormatText{}, OptionLimit(10))
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

	r, err := NewReader(FormatText{}, OptionMatch([]JournalEntryMatch{
		{
			Field: "CUSTOM_FIELD",
			Value: first,
		},
	}))
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

	r, err := NewReader(FormatJSON{}, OptionMatch([]JournalEntryMatch{
		{
			Field: "CUSTOM_FIELD",
			Value: uniq,
		},
	}), OptionSkipNext(2))
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
		expectedString := fmt.Sprintf("index-%d", size)
		if value != expectedString {
			t.Fatalf("Expected: %s. Got %s", expectedString, value)
		}
		size++
	}
	if size != 4 {
		t.Fatalf("Must have 4 entries. Got %d", size)
	}
}

func TestOptionMatchOR(t *testing.T) {
	str1 := getUniqueString()
	str2 := getUniqueString()
	journal.Send(str1, journal.PriInfo, map[string]string{"PROP1": str1})
	journal.Send(str2, journal.PriInfo, map[string]string{"PROP2": str2})

	// wait for journal entries to commit
	time.Sleep(time.Millisecond * 100)

	r, err := NewReader(FormatJSON{}, OptionMatchOR([]JournalEntryMatch{
		{
			Field: "PROP1",
			Value: str1,
		},
		{
			Field: "PROP2",
			Value: str2,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	var size int
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		size++
	}

	if size != 2 {
		t.Fatalf("Expecting to find 2 entries, got %d", size)
	}
}

func TestFollow(t *testing.T) {
	id := getUniqueString()
	ctx, cancel := context.WithCancel(context.Background())
	ack := make(chan bool)

	go func() {
		for i := 0; i < 10; i++ {

			journal.Send(fmt.Sprintf("test %s - %d", id, i), journal.PriInfo, map[string]string{"TEST_ID": id})
			<-ack
		}
		cancel()
	}()

	r, err := NewReader(FormatJSON{}, OptionMatchOR([]JournalEntryMatch{
		{
			Field: "TEST_ID",
			Value: id,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	type logEntry struct {
		Fields map[string]string `json:"fields"`
	}

	messageCounter := 0
	for {
		select {
		case <-ctx.Done():
			if messageCounter != 10 {
				t.Fatalf("expecting to validate 10 log entries. Validated only %d", messageCounter)
			}
			return
		case <-time.After(time.Second):
			t.Fatal("too much time to read journal")
		default:
			buf := new(bytes.Buffer)
			r.Follow(time.Second, buf)

			entry := &logEntry{}
			err = json.NewDecoder(buf).Decode(entry)
			if err != nil {
				if err == io.EOF {
					continue
				}
				t.Fatal(err)
			}
			expectedMessage := fmt.Sprintf("test %s - %d", id, messageCounter)
			if entry.Fields["MESSAGE"] != expectedMessage {
				t.Fatalf("expecting message %s. Got %s", expectedMessage, entry.Fields["MESSAGE"])
			}
			messageCounter++

			select {
			case ack <- true:
			case <-time.After(time.Second):
				t.Fatal("too much time to send ack")
			}
		}
	}
}
