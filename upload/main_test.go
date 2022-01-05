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
	fakeParams.Add("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	fakeParams.Add("X-Amz-Credential", "XXXXXXXXXXXXXXX")
	fakeParams.Add("X-Amz-Security-Token", "ZZZZZZZZZZZZZZZ")
	fakeParams.Add("X-Amz-Signature", "YYYYYYYYYYYYYYY")
	fakeParams.Add("X-Amz-Expires", "300")
	fakeParams.Add("X-Amz-Date", "20220104T215629Z")
	fakeParams.Add("X-Amz-SignedHeaders", "content-type;content-md5;host;x-amz-meta-accuracy;x-amz-meta-latitude;x-amz-meta-longitude;x-amz-meta-user-hash")
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
					"email":       "user@eaxmple.com",
					"name":        "First Last",
					"custom:hash": "b2ca42478035dbd6208df19f87914f3499f851279d14f956c75d0aeda2d9e4d7",
				},
			},
		},
		Body: string(body),
	}
}

type bodyResult struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
	ID   string `json:"id"`
}

func Test_uploadHandler(t *testing.T) {
	c := require.New(t)

	response, err := uploadHandler(uploadRequest())
	c.Nil(err)
	c.NotNil(response)

	body := &bodyResult{}

	err = json.Unmarshal([]byte(response.Body), body)
	c.Nil(err)

	c.Equal(body.Hash, "b2ca42478035dbd6208df19f87914f3499f851279d14f956c75d0aeda2d9e4d7")

	u, _ := url.Parse(body.URL)
	c.Equal(u.Scheme, "https")
	c.Equal(u.Host, "s3.eu-west-1.amazonaws.com")
	c.Equal(u.Path, "/my-bucket-test/"+body.ID+".jpe")

	p, _ := url.ParseQuery(u.RawQuery)
	c.Equal(p["X-Amz-Algorithm"][0], "AWS4-HMAC-SHA256")
	c.Equal(p["X-Amz-Credential"][0], "XXXXXXXXXXXXXXX")
	c.Equal(p["X-Amz-Date"][0], "20220104T215629Z")
	c.Equal(p["X-Amz-Expires"][0], "300")
	c.Equal(p["X-Amz-Security-Token"][0], "ZZZZZZZZZZZZZZZ")
	c.Equal(p["X-Amz-SignedHeaders"][0], "content-type;content-md5;host;x-amz-meta-accuracy;x-amz-meta-latitude;x-amz-meta-longitude;x-amz-meta-user-hash")
	c.Equal(p["X-Amz-Signature"][0], "YYYYYYYYYYYYYYY")
}
