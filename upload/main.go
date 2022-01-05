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
	return req.Presign(5 * time.Minute)
}

type GPSdata struct {
	Latitude  float64 `json:"Latitude"`
	Longitude float64 `json:"Longitude"`
	Accuracy  float64 `json:"Accuracy"`
}

type MetadataRequest struct {
	ContentType string  `json:"Content-Type"`
	ContentMD5  string  `json:"Content-MD5"`
	GPS         GPSdata `json:"GPS"`
}

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Hash  string `json:"custom:hash"`
}

func getUser(request *events.APIGatewayProxyRequest) (User, error) {
	input := request.RequestContext.Authorizer["claims"]
	output := User{}

	jsonStr, err := json.Marshal(input)
	if err != nil {
		return output, err
	}

	err = json.Unmarshal(jsonStr, &output)

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
		Bucket:      aws.String(uploadBucket),
		Key:         aws.String(imageID + extensions[0]),
		ContentType: aws.String(metaRequest.ContentType),
		ContentMD5:  aws.String(metaRequest.ContentMD5),
		Metadata: aws.StringMap(map[string]string{
			"Accuracy":  strconv.FormatFloat(metaRequest.GPS.Accuracy, 'G', -1, 64),
			"Latitude":  strconv.FormatFloat(metaRequest.GPS.Latitude, 'G', -1, 64),
			"Longitude": strconv.FormatFloat(metaRequest.GPS.Longitude, 'G', -1, 64),
			"User-Hash": User.Hash,
		}),
	})

	body, _ := json.Marshal(map[string]interface{}{
		"url":  string(urlString),
		"hash": User.Hash,
		"id":   string(imageID),
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
