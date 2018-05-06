package lib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
)

func decryptApiKey() string {
	kmsClient := kms.New(session.New())
	decodedBytes, err := base64.StdEncoding.DecodeString(FreshBooksEncryptedAPIKey())
	if err != nil {
		log.Fatal("Failed to base64 decode encrypted string: %s", err)
	}
	input := &kms.DecryptInput{CiphertextBlob: decodedBytes}
	response, err := kmsClient.Decrypt(input)
	if err != nil {
		log.Fatal("Failed to decrypt key: %s", err)
	}
	// Plaintext is a byte array, so convert to string
	return string(response.Plaintext[:])
}

func validateSlackToken(givenToken string) bool {
	expectedToken := FreshBooksSlackVerificationToken()
	return (expectedToken == "") || (givenToken == expectedToken)
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	params, err := url.ParseQuery(request.Body)
	if err != nil {
		log.Fatal("Failed to parse request body")
	}

	if !validateSlackToken(params["token"][0]) {
		return events.APIGatewayProxyResponse{StatusCode: 401}, nil
	}

	api := AuthenticateFreshbooksApi(FreshBooksOrganizationName(), decryptApiKey())
	hours := HourBundlesForActiveProjects(api)

	sort.Slice(hours, func(i, j int) bool {
		return hours[i].Project.Name < hours[j].Project.Name
	})

	attachments_fields := make([]map[string]string, 0)
	for _, h := range hours {
		hoursLeft := math.Max(0, h.Project.HourBudget-h.WorkedHours)
		field := map[string]string{
			"title": h.Project.Name,
			"value": fmt.Sprintf("%.2f of %.2f hours", hoursLeft, h.Project.HourBudget),
			"short": "true",
		}
		attachments_fields = append(attachments_fields, field)
	}

	attachments := map[string]interface{}{
		"fallback":    "Freshbot",
		"color":       "#36a64f",
		"title":       "Hours Left on Active Projects",
		"title_link":  fmt.Sprintf("https://%s.freshbooks.com", FreshBooksOrganizationName()),
		"fields":      attachments_fields,
		"footer":      "FreshBooks API",
		"footer_icon": "https://www.freshbooks.com/favicon.ico",
		"ts":          time.Now().Unix(),
	}

	body := map[string]interface{}{
		"response_type": "ephemeral",
		"attachments":   []interface{}{attachments},
	}

	body_json, err := json.Marshal(body)
	if err != nil {
		log.Fatal("Failed to encode body: %s", err)
	}

	return events.APIGatewayProxyResponse{
		Body:       string(body_json),
		StatusCode: 200,
	}, nil
}
