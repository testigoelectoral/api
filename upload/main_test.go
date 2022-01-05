package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/stretchr/testify/require"
)

type mockS3Uploader struct {
}

func (u *mockS3Uploader) PresigUrl(input *s3.PutObjectInput) (string, http.Header, error) { //nolint:revive
	fakePath := "https://s3.eu-west-1.amazonaws.com/" + *input.Bucket + "/" + *input.Key + "?"
	fakeParams := url.Values{}
	fakeParams.Add("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	fakeParams.Add("X-Amz-Credential", "XXXXXXXXXXXXXXX")
	fakeParams.Add("X-Amz-Security-Token", "ZZZZZZZZZZZZZZZ")
	fakeParams.Add("X-Amz-Signature", "YYYYYYYYYYYYYYY")
	fakeParams.Add("X-Amz-Expires", "900")
	fakeParams.Add("X-Amz-Date", "20220104T215629Z")
	fakeParams.Add("X-Amz-SignedHeaders", "cache-control;content-type;host;x-amz-meta-accuracy;x-amz-meta-contenttype;x-amz-meta-latitude;x-amz-meta-longitude;x-amz-meta-userhash")
	fakeURL := fakePath + fakeParams.Encode()

	headers := http.Header{}
	headers.Add("x-amz-meta-contenttype", *input.Metadata["contentType"])
	headers.Add("x-amz-meta-userhash", *input.Metadata["userHash"])
	headers.Add("x-amz-meta-latitude", *input.Metadata["latitude"])
	headers.Add("x-amz-meta-longitude", *input.Metadata["longitude"])
	headers.Add("x-amz-meta-accuracy", *input.Metadata["accuracy"])
	headers.Add("cache-control", *input.CacheControl)
	headers.Add("content-type", *input.ContentType)
	headers.Add("host", "s3.eu-west-1.amazonaws.com")

	return fakeURL, headers, nil
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
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	ID      string              `json:"id"`
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
	c.Equal(p["X-Amz-Algorithm"][0], "AWS4-HMAC-SHA256")
	c.Equal(p["X-Amz-Credential"][0], "XXXXXXXXXXXXXXX")
	c.Equal(p["X-Amz-Security-Token"][0], "ZZZZZZZZZZZZZZZ")
	c.Equal(p["X-Amz-Signature"][0], "YYYYYYYYYYYYYYY")
	c.Equal(p["X-Amz-Expires"][0], "900")
	c.Equal(p["X-Amz-Date"][0], "20220104T215629Z")
	c.Equal(p["X-Amz-SignedHeaders"][0], "cache-control;content-type;host;x-amz-meta-accuracy;x-amz-meta-contenttype;x-amz-meta-latitude;x-amz-meta-longitude;x-amz-meta-userhash")

	c.Equal(body.Headers["Cache-Control"][0], "max-age=31557600")
	c.Equal(body.Headers["Content-Type"][0], "image/jpeg")
	c.Equal(body.Headers["Host"][0], "s3.eu-west-1.amazonaws.com")
	c.Equal(body.Headers["X-Amz-Meta-Accuracy"][0], "15.391")
	c.Equal(body.Headers["X-Amz-Meta-Contenttype"][0], "image/jpeg")
	c.Equal(body.Headers["X-Amz-Meta-Longitude"][0], "-74.078918")
	c.Equal(body.Headers["X-Amz-Meta-Userhash"][0], "b2ca42478035dbd6208df19f87914f3499f851279d14f956c75d0aeda2d9e4d7")
}
