package container

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type AuthInfo struct {
	Authorization string `json:"authorization"`
}

// TagInfo
type TagInfo struct {
	Name             string `json:"name"`
	Reversion        bool   `json:"reversion"`
	Start_ts         int64  `json:"start_ts"`
	End_ts           int64  `json:"end_ts"`
	Image_Id         string `json:"image_id"`
	Last_modified    string `json:"last_modified"`
	Expiration       string `json:"expiration"`
	manifest_digest  string `json:"manifest_digest"`
	Docker_image_id  string `json:"docker_image_id"`
	Is_manifest_list bool   `json:"is_manifest_list"`
	Size             int64  `json:"size"`
}

type TagsResult struct {
	has_additional bool      `json:"has_additional"`
	page           int       `json:"page"`
	Tags           []TagInfo `json:"tags"`
}

// PodmanCLI provides function to run the docker command
type QuayCLI struct {
	EndPointPre   string
	Authorization string
}

func NewQuayCLI() *QuayCLI {
	newclient := &QuayCLI{}
	newclient.EndPointPre = "https://quay.io/api/v1/repository/"
	authString := ""
	authFilepath := ""
	if strings.Compare(os.Getenv("QUAY_AUTH_FILE"), "") != 0 {
		authFilepath = os.Getenv("QUAY_AUTH_FILE")
	} else {
		_, fullFilename, _, _ := runtime.Caller(0)
		authFilepath = path.Dir(path.Dir(path.Dir(path.Dir(path.Dir(fullFilename))))) + "/secrets/quay/quay_auth.json"
	}
	e2e.Logf("get quay auth from file %s", authFilepath)
	if _, err := os.Stat(authFilepath); os.IsNotExist(err) {
		e2e.Logf("auth file does not exist")
	} else {
		content, err := ioutil.ReadFile(authFilepath)
		if err != nil {
			e2e.Logf("File reading error")
		} else {
			var authJson AuthInfo
			if err := json.Unmarshal(content, &authJson); err != nil {
				e2e.Logf("parser json error, json content is %s", string(content))
			} else {
				authString = "Bearer " + authJson.Authorization
			}
		}
	}
	if strings.Compare(os.Getenv("QUAY_AUTH"), "") != 0 {
		e2e.Logf("get quay auth from env QUAY_AUTH")
		authString = "Bearer " + os.Getenv("QUAY_AUTH")
	}
	e2e.Logf("authstring %s", string(authString))
	newclient.Authorization = authString
	return newclient
}

// DeleteTag will delete the image
func (c *QuayCLI) DeleteTag(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("DELETE", endpoint, nil)
	if strings.Compare(c.Authorization, "") != 0 {
		reqest.Header.Add("Authorization", c.Authorization)
	}

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode != 204 {
		e2e.Logf("delete %s failed, response code is %d", imageIndex, response.StatusCode)
		return false, nil
	}
	return true, nil
}

func (c *QuayCLI) CheckTagNotExist(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex + "/images"
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	reqest.Header.Add("Authorization", c.Authorization)

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode == 404 {
		e2e.Logf("tag %s not exist", imageIndex)
		return true, nil
	} else {
		contents, _ := ioutil.ReadAll(response.Body)
		e2e.Logf("responce is %s", string(contents))
		return false, nil
	}
}

func (c *QuayCLI) GetTagNameList(imageIndex string) ([]string, error) {
	var TagNameList []string
	tags, err := c.GetTags(imageIndex)
	if err != nil {
		return TagNameList, err
	}
	for _, tagIndex := range tags {
		TagNameList = append(TagNameList, tagIndex.Name)
	}
	return TagNameList, nil
}

func (c *QuayCLI) GetTags(imageIndex string) ([]TagInfo, error) {
	var result []TagInfo
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Split(imageIndex, ":")[0] + "/tag/"
	}
	if strings.Contains(imageIndex, "/tag/") {
		imageIndex = strings.Split(imageIndex, "tag/")[0] + "tag/"
	}
	endpoint := c.EndPointPre + imageIndex
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	reqest.Header.Add("Authorization", c.Authorization)
	if err != nil {
		return result, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return result, err
	}
	e2e.Logf("%s", response.Status)
	if response.StatusCode != 200 {
		e2e.Logf("get %s failed, response code is %d", imageIndex, response.StatusCode)
		return result, fmt.Errorf("return code is %d, not 200", response.StatusCode)
	} else {
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return result, err
		}
		//e2e.Logf(string(contents))
		//unmarshal json file
		var TagsResultOut TagsResult
		if err := json.Unmarshal(contents, &TagsResultOut); err != nil {
			return result, err
		}
		result = TagsResultOut.Tags
		return result, nil
	}
}