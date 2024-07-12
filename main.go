package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"mime/quotedprintable"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DusanKasan/parsemail"
	"golang.org/x/net/html"
)

var OUTDIR = "./out/"

func main() {
	createOutDir()
	participants, volunteers := generateParticipantLists()

	sortByRegisteredAt(participants)
	generateParticipantsCSV(participants)
	generateVolunteersCSV(volunteers)
	generateEmailLists(participants, volunteers)

	raceNumber := []string{}
	for _, participant := range participants {
		raceNumber = append(raceNumber, participant.raceNumber)
	}

	sort.Slice(raceNumber, func(i, j int) bool {
		a, _ := strconv.Atoi(raceNumber[i])
		b, _ := strconv.Atoi(raceNumber[j])
		return a < b
	})

	fmt.Println(raceNumber)

	fmt.Println("Number of participants: ", len(participants))
	fmt.Println("Number of volunteers: ", len(volunteers))
}

func createOutDir() {
	err := os.MkdirAll(OUTDIR, 0o755)
	if err != nil {
		log.Fatal("creating output directory: ", err.Error())
	} else {
		fmt.Println("Output directory './out' created")
	}
}

func generateParticipantsCSV(participants []ParticipantInfo) {
	fmt.Println("Generating Participant CSV")
	header := []string{"name", "email", "category", "pronouns", "racenumber", "city/team", "arrival", "departure", "registered_at", "message"}
	file, err := os.Create(OUTDIR + "participants-ecmc24.csv")
	if err != nil {
		log.Fatalln("Couldn't create file", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	err = w.Write(header)
	if err != nil {
		log.Fatalln("Couldn't write header to file", err)
	}

	var data [][]string
	for _, v := range participants {
		row := []string{
			v.name,
			v.email,
			v.category,
			v.pronouns,
			v.raceNumber,
			v.cityOrCompany,
			v.arriving,
			v.departing,
			v.registeredAt.String(),
			v.message,
		}
		if err := w.Write(row); err != nil {
			log.Fatalln("Couldn't write row to file", err)
		}

		if err := w.WriteAll(data); err != nil {
			log.Fatalln("Couldn't write rows to file", err)
		}
	}
}

func generateVolunteersCSV(volunteers []ParticipantInfo) {
	fmt.Println("Generating volunteers CSV")
	header := []string{"name", "email", "category", "pronouns", "city/team", "arrival", "departure", "registered_at", "message"}
	file, err := os.Create(OUTDIR + "volunteers-ecmc24.csv")
	if err != nil {
		log.Fatalln("Couldn't create file", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	err = w.Write(header)
	if err != nil {
		log.Fatalln("Couldn't write header to file", err)
	}

	var data [][]string
	for _, v := range volunteers {
		row := []string{
			v.name,
			v.email,
			v.category,
			v.pronouns,
			v.cityOrCompany,
			v.arriving,
			v.departing,
			v.registeredAt.String(),
			v.message,
		}
		if err := w.Write(row); err != nil {
			log.Fatalln("Couldn't write row to file", err)
		}

		if err := w.WriteAll(data); err != nil {
			log.Fatalln("Couldn't write rows to file", err)
		}
	}
}

func generateEmailLists(participants []ParticipantInfo, volunteers []ParticipantInfo) {
	fmt.Println("Generating email lists")

	volunteerEmails := []string{}
	for _, volunteer := range volunteers {
		volunteerEmails = append(volunteerEmails, volunteer.email)
	}
	volunteerEmails = deduplicateList(volunteerEmails)

	participantEmails := []string{}
	for _, participant := range participants {
		participantEmails = append(participantEmails, participant.email)
	}
	participantEmails = deduplicateList(participantEmails)

	allEmails := append(volunteerEmails, participantEmails...)
	allEmails = deduplicateList(allEmails)

	volunteerEmailFile, err := os.Create(OUTDIR + "volunteer-emails-ecmc24.txt")
	if err != nil {
		log.Fatalln("Couldn't create file", err)
	}
	defer volunteerEmailFile.Close()

	participantEmailFile, err := os.Create(OUTDIR + "participant-emails-ecmc24.txt")
	if err != nil {
		log.Fatalln("Couldn't create file", err)
	}
	defer participantEmailFile.Close()

	allEmailFile, err := os.Create(OUTDIR + "all-emails-ecmc24.txt")
	if err != nil {
		log.Fatalln("Couldn't create file", err)
	}
	defer allEmailFile.Close()

	volunteerEmailString := strings.Join(volunteerEmails, ",")
	_, err = volunteerEmailFile.WriteString(volunteerEmailString)
	if err != nil {
		log.Fatal(err)
	}

	participantEmailString := strings.Join(participantEmails, ",")
	_, err = participantEmailFile.WriteString(participantEmailString)
	if err != nil {
		log.Fatal(err)
	}

	allEmailString := strings.Join(allEmails, ",")
	_, err = allEmailFile.WriteString(allEmailString)
	if err != nil {
		log.Fatal(err)
	}
}

func generateParticipantLists() ([]ParticipantInfo, []ParticipantInfo) {
	dirPath := "./ecmc-form-submissions"

	participants := []ParticipantInfo{}
	volunteers := []ParticipantInfo{}

	dir, err := os.Open(dirPath)
	if err != nil {
		log.Fatalf("Error opening directory: %v\n", err)
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1) // -1 means return all entries
	if err != nil {
		log.Fatalf("Error reading directory: %v\n", err)
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.Mode().IsRegular() {
			filePath := filepath.Join(dirPath, fileInfo.Name())

			// Open the file
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Printf("Error opening file %s: %v\n", filePath, err)
				continue
			}
			defer file.Close()

			email, err := parsemail.Parse(file)
			if err != nil {
				log.Fatal(err)
			}
			results := parseHTML(email.HTMLBody)

			if strings.Contains(email.Subject, "2") {
				volunteer := volunteerInfoFromResult(results, email.ReplyTo[0].Address, email.Date.UTC())
				volunteers = append(volunteers, volunteer)
			} else {
				participant := participantInfoFromResult(results, email.ReplyTo[0].Address, email.Date.UTC())
				participants = append(participants, participant)
			}

		}
	}
	return participants, volunteers
}

type ParticipantInfo struct {
	registeredAt  time.Time
	name          string
	email         string
	category      string
	pronouns      string
	message       string
	cityOrCompany string
	raceNumber    string
	arriving      string
	departing     string
}

func volunteerInfoFromResult(result []string, email string, registeredAt time.Time) ParticipantInfo {
	var arriving, departing string
	if len(result) > 6 {
		arriving = cleanLine(result[6])
		departing = cleanLine(result[7])
	}
	return ParticipantInfo{
		registeredAt:  registeredAt,
		name:          decodeMessage(cleanLine(result[0])),
		email:         email,
		category:      cleanLine(result[2]),
		pronouns:      cleanLine(result[3]),
		message:       decodeMessage(cleanLine(result[4])),
		cityOrCompany: cleanLine(result[5]),
		raceNumber:    "",
		arriving:      arriving,
		departing:     departing,
	}
}

func participantInfoFromResult(result []string, email string, registeredAt time.Time) ParticipantInfo {
	var arriving, departing string
	if len(result) > 7 {
		arriving = cleanLine(result[7])
		departing = cleanLine(result[8])
	}
	return ParticipantInfo{
		registeredAt:  registeredAt,
		name:          decodeMessage(cleanLine(result[0])),
		email:         email,
		category:      cleanLine(result[2]),
		pronouns:      cleanLine(result[3]),
		message:       decodeMessage(cleanLine(result[4])),
		cityOrCompany: cleanLine(result[5]),
		raceNumber:    cleanLine(result[6]),
		arriving:      arriving,
		departing:     departing,
	}
}

// Function to extract text content from an HTML node
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type != html.ElementNode {
		return ""
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}

// Function to check if a node has a span sibling and return the span text if it exists
func getSpanSiblingText(n *html.Node) (string, bool) {
	for sibling := n.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == html.ElementNode && sibling.Data == "span" {
			return extractText(sibling), true
		}
	}
	return "", false
}

// Function to extract specific information from the HTML
func parseHTML(htmlStr string) []string {
	var results []string

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		fmt.Printf("Error parsing HTML: %v\n", err)
		return results
	}

	// Example: Find and store all <b> elements with a <span> sibling, stop parsing when "Sent via form submission from " is found
	var f func(*html.Node)
	stopParsing := false
	f = func(n *html.Node) {
		if stopParsing {
			return
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if strings.HasPrefix(text, "Sent via form submission from") {
				stopParsing = true // Set flag to stop parsing
				return
			}
		}

		if n.Type == html.ElementNode && n.Data == "b" {
			if spanText, ok := getSpanSiblingText(n); ok {
				bText := extractText(n)
				results = append(results, fmt.Sprintf("%s %s", bText, spanText))
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return results
}

func cleanLine(line string) string {
	return strings.TrimSpace(strings.Join(strings.Split(line, ":")[1:], ""))
}

func deduplicateList(input []string) []string {
	encountered := map[string]bool{}
	deduplicated := []string{}

	for _, v := range input {
		if !encountered[v] {
			encountered[v] = true
			deduplicated = append(deduplicated, v)
		}
	}
	return deduplicated
}

func decodeMessage(input string) string {
	// Quoted-printable encoded string

	// Replace '=' followed by newline with nothing (to remove soft line breaks)
	input = strings.ReplaceAll(input, "=\n", "")

	// Decode quoted-printable to UTF-8
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(input)))
	if err != nil {
		log.Fatal("Error decoding:", err)
	}

	// Convert bytes to string
	decodedString := string(decoded)

	return decodedString
}

func sortByRegisteredAt(participants []ParticipantInfo) []ParticipantInfo {
	sort.Slice(participants, func(i, j int) bool {
		return participants[i].registeredAt.Before(participants[j].registeredAt)
	})
	return participants
}
