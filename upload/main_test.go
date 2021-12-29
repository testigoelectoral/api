package main

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/stretchr/testify/require"
)

type mockS3Uploader struct {
}

func (u *mockS3Uploader) PresigUrl(input *s3.PutObjectInput) (string, error) { //nolint:revive
	fakePath := "https://s3.eu-west-1.amazonaws.com/" + *input.Bucket + "/" + *input.Key + "?"
	fakeParams := url.Values{}
	fakeParams.Add("AWSAccessKeyId", "XXXXXXXXXXXXXXX")
	fakeParams.Add("Cache-Control", *input.CacheControl)
	fakeParams.Add("Content-Type", *input.ContentType)
	fakeParams.Add("Signature", "YYYYYYYYYYYYYYY")
	fakeParams.Add("x-amz-meta-contenttype", *input.Metadata["contentType"])
	fakeParams.Add("x-amz-meta-userhash", *input.Metadata["userHash"])
	fakeParams.Add("x-amz-meta-latitude", *input.Metadata["latitude"])
	fakeParams.Add("x-amz-meta-longitude", *input.Metadata["longitude"])
	fakeParams.Add("x-amz-meta-accuracy", *input.Metadata["accuracy"])
	fakeURL := fakePath + fakeParams.Encode()

	return fakeURL, nil
}

func init() {
	uploadBucket = "my-bucket-test"
	s3Service = &mockS3Uploader{}
}

func uploadRequest() *events.APIGatewayProxyRequest {
	body, _ := json.Marshal(MetadataRequest{
		ContentType: "image/jpeg",
		GPS: GPSdata{
			Latitude:  4.595696,
			Longitude: -74.078918,
			Accuracy:  15.391,
		},
	})

	return &events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		RequestContext: events.APIGatewayProxyRequestContext{
			Authorizer: map[string]interface{}{
				"claims": map[string]interface{}{
					"email": "user@eaxmple.com",
					"name":  "First Last",
					"hash":  "b2ca42478035dbd6208df19f87914f3499f851279d14f956c75d0aeda2d9e4d7",
				},
			},
		},
		Body: string(body),
	}
}

type bodyResult struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

func Test_uploadHandler(t *testing.T) {
	c := require.New(t)

	response, err := uploadHandler(uploadRequest())
	c.Nil(err)
	c.NotNil(response)

	body := &bodyResult{}

	err = json.Unmarshal([]byte(response.Body), body)
	c.Nil(err)

	u, _ := url.Parse(body.URL)
	c.Equal(u.Scheme, "https")
	c.Equal(u.Host, "s3.eu-west-1.amazonaws.com")
	c.Equal(u.Path, "/my-bucket-test/"+body.ID+".jpe")

	p, _ := url.ParseQuery(u.RawQuery)
	c.Equal(p["AWSAccessKeyId"][0], "XXXXXXXXXXXXXXX")
	c.Equal(p["Cache-Control"][0], "max-age=31557600")
	c.Equal(p["Content-Type"][0], "image/jpeg")
	c.Equal(p["Signature"][0], "YYYYYYYYYYYYYYY")
	c.Equal(p["x-amz-meta-userhash"][0], "b2ca42478035dbd6208df19f87914f3499f851279d14f956c75d0aeda2d9e4d7")
	c.Equal(p["x-amz-meta-contenttype"][0], "image/jpeg")
	c.Equal(p["x-amz-meta-latitude"][0], "4.595696")
	c.Equal(p["x-amz-meta-longitude"][0], "-74.078918")
	c.Equal(p["x-amz-meta-accuracy"][0], "15.391")
}
