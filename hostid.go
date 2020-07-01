package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"
)

var availabilityZoneRegexp *regexp.Regexp

func init() {
	availabilityZoneRegexp = regexp.MustCompile(`^(?P<region>[a-z]+(?:-[a-z]+)*-[1-9][0-9]*)(?P<az>(?P<traditional>[a-z]+)|-(?P<localzone>[a-z]+-[1-9][0-9]*[a-z]+))$`)
}

// GetHostID returns a host ID to use for Kinesis.
func GetHostID() string {
	if cachedHostID != "" {
		return cachedHostID
	}

	ecsMetadataURI := os.Getenv("ECS_CONTAINER_METADATA_URI_V4")
	if ecsMetadataURI != "" {
		hostID, err := getHostIDFromECS(ecsMetadataURI + "/task")
		if err == nil {
			cachedHostID = hostID
			return cachedHostID
		}

		fmt.Fprintf(os.Stderr, "Failed to get task ARN from ECS metadata v4 endpoint: %v\n", err)
	}

	ecsMetadataURI = os.Getenv("ECS_CONTAINER_METADATA_URI")
	if ecsMetadataURI != "" {
		hostID, err := getHostIDFromECS(ecsMetadataURI + "/task")
		if err == nil {
			cachedHostID = hostID
			return cachedHostID
		}

		fmt.Fprintf(os.Stderr, "Failed to get task ARN from ECS metadata v3 endpoint: %v\n", err)
	}

	hostID, errECS := getHostIDFromECS("http://169.254.170.2/v2/metadata")
	if errECS == nil {
		cachedHostID = hostID
		return cachedHostID
	}

	hostID, errEC2 := getHostIDFromEC2()
	if errEC2 == nil {
		cachedHostID = hostID
		return cachedHostID
	}

	hostID, errIF := getHostIDFromInterfaces()
	if errIF == nil {
		cachedHostID = hostID
		return cachedHostID
	}

	hostID, errRand := getRandomHostID()
	if errRand == nil {
		cachedHostID = hostID
		return cachedHostID
	}

	fmt.Fprintf(os.Stderr, "Failed to get task ARN from ECS metadata v2 endpoint: %v\n", errECS)
	fmt.Fprintf(os.Stderr, "Failed to get instance ID from EC2 metadata endpoint: %v\n", errEC2)
	fmt.Fprintf(os.Stderr, "Failed to get IP address from network interface: %v\n", errIF)
	fmt.Fprintf(os.Stderr, "Failed to get random host ID: %v\n", errRand)
	panic("Unable to obtain a valid host ID")
}

func getHostIDFromECS(taskURL string) (string, error) {
	httpClient := http.Client{Timeout: 500 * time.Millisecond}
	response, err := httpClient.Get(taskURL)
	if err != nil {
		return "", err
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("Failed to read ECS metadata endpoint: %s", response.Status)
	}

	result := make(map[string]interface{})
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&result)
	if err != nil {
		return "", fmt.Errorf("Failed to parse ECS metadata result as JSON: %v", err)
	}

	taskARNAny, found := result["TaskARN"]
	if !found {
		return "", fmt.Errorf("ECS metadata does not contain a valid TaskARN element")
	}

	taskARN, castOK := taskARNAny.(string)
	if !castOK || taskARN == "" {
		return "", fmt.Errorf("ECS metadata does not contain a valid TaskARN element")
	}

	return taskARN, nil
}

func getEC2Metadata(relURL string) (string, error) {
	httpClient := http.Client{Timeout: 500 * time.Millisecond}
	result := make([]byte, 65536, 65536)

	response, err := httpClient.Get("http://169.254.169.254/2019-10-01/meta-data/" + relURL)
	if err != nil {
		return "", err
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("Failed to read EC2 metadata endpoint: %s", response.Status)
	}

	nRead, err := response.Body.Read(result)
	if err != nil {
		return "", fmt.Errorf("Failed to read EC2 metadata %s: %v", relURL, err)
	}

	if nRead == 0 {
		return "", fmt.Errorf("EC2 %s is empty", relURL)
	}

	return string(result[:nRead]), nil
}

func getHostIDFromEC2() (string, error) {
	partition, err := getEC2Metadata("services/partition")
	if err != nil {
		return "", err
	}

	identity, err := getEC2Metadata("identity-credentials/ec2/info")
	if err != nil {
		return "", err
	}
	identityDoc := make(map[string]interface{})
	err = json.Unmarshal([]byte(identity), &identityDoc)
	if err != nil {
		return "", fmt.Errorf("Unable to parse instance identity credentials as JSON: %v", err)
	}
	accountIDAny, found := identityDoc["AccountId"]
	accountID, castOk := accountIDAny.(string)
	if !found || !castOk || accountID == "" {
		return "", fmt.Errorf("AccountId not present in instance identity credentials")
	}

	availabilityZone, err := getEC2Metadata("placement/availability-zone")
	if err != nil {
		return "", err
	}

	instanceID, err := getEC2Metadata("instance-id")
	if err != nil {
		return "", err
	}

	region := availabilityZoneRegexp.FindStringSubmatch(availabilityZone)[1]

	return "arn:" + partition + ":ec2:" + region + ":" + accountID + ":instance/" + instanceID, nil
}

func getHostIDFromInterfaces() (string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err == nil && ip.IsGlobalUnicast() {
			return "ip-address:" + ip.String(), nil
		}
	}

	return "", fmt.Errorf("No valid IP addresses found")
}

func getRandomHostID() (string, error) {
	randomBytes := make([]byte, 16, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	hostID := make([]byte, hex.EncodedLen(16))
	hex.Encode(hostID, randomBytes)

	return "uuid:" + string(hostID), nil
}
