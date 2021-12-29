package main

import (
	"encoding/json"
	"mime"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/google/uuid"

	// TODO: Remove this external libray?
	"github.com/mitchellh/mapstructure"
)

var (
	uploadBucket string
	s3Service    actualUploader
)

type actualUploader interface {
	PresigUrl(*s3.PutObjectInput) (string, error)
}

type s3Uploader struct {
	service s3iface.S3API
}

func (u *s3Uploader) PresigUrl(input *s3.PutObjectInput) (string, error) { //nolint:revive
	req, _ := u.service.PutObjectRequest(input)
	return req.Presign(15 * time.Minute)
}

type GPSdata struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"`
}

type MetadataRequest struct {
	ContentType string  `json:"contentType"`
	GPS         GPSdata `json:"gps"`
}

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Hash  string `json:"hash"`
}

func getUser(request *events.APIGatewayProxyRequest) (User, error) {
	input := request.RequestContext.Authorizer["claims"]
	output := User{}

	err := mapstructure.Decode(input, &output)

	return output, err
}

func uploadHandler(request *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	var err error

	metaRequest := &MetadataRequest{}

	err = json.Unmarshal([]byte(request.Body), metaRequest)
	if err != nil {
		return nil, err
	}

	User, err := getUser(request)
	if err != nil {
		return nil, err
	}

	imageID := uuid.New().String()

	extensions, err := mime.ExtensionsByType(metaRequest.ContentType)
	if err != nil {
		return nil, err
	}

	urlString, err := s3Service.PresigUrl(&s3.PutObjectInput{
		Bucket:       aws.String(uploadBucket),
		Key:          aws.String(imageID + extensions[0]),
		ContentType:  aws.String(metaRequest.ContentType),
		CacheControl: aws.String("max-age=31557600"),
		Metadata: aws.StringMap(map[string]string{
			"contentType": metaRequest.ContentType,
			"userHash":    User.Hash,
			"latitude":    strconv.FormatFloat(metaRequest.GPS.Latitude, 'G', -1, 64),
			"longitude":   strconv.FormatFloat(metaRequest.GPS.Longitude, 'G', -1, 64),
			"accuracy":    strconv.FormatFloat(metaRequest.GPS.Accuracy, 'G', -1, 64),
		}),
	})

	body, _ := json.Marshal(map[string]interface{}{
		"url": string(urlString),
		"id":  string(imageID),
	})

	uploadHandlerResult := &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(body),
	}

	return uploadHandlerResult, err
}

func init() {
	uploadBucket = os.Getenv("UPLOAD_BUCKET")

	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	s3Service = &s3Uploader{
		service: s3.New(sess),
	}
}

func main() {
	lambda.Start(uploadHandler)
}
