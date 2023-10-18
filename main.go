package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	outputFile = "output.csv"
	inputFile  = "input.csv"
)

type ApiResponse struct {
	Success bool                     `json:"success"`
	Data    []map[string]interface{} `json:"data"`
	Meta    PageData                 `json:"meta"`
}

type PageData struct {
	Page  int `json:"page"`
	Total int `json:"totalPage"`
}

func main() {

	configPath := flag.String("conf", "config.yml", "path to config file")
	apiURL := flag.String("url", "", "API resource URL to fetch data from")
	apiMethod := flag.String("method", "GET", "HTTP method (GET, POST, etc.)")
	flag.Parse()

	if *apiURL == "" {
		fmt.Println("Please provide an API URL.")
		return
	}

	conf, err := GetConfig(*configPath)
	if err != nil {
		fmt.Println("reading config file:", err)
		return
	}

	baseUrl := conf.BaseUrl
	if baseUrl == "" {
		fmt.Println("Please provide a base URL in the configuration file.")
		return
	}

	_ = os.Remove("errors.log")
	file, err := os.OpenFile("errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("opening or creating log file: %v\n", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("closing log file:", err)
			return
		}
	}(file)
	os.Stdout = file

	RemoveFiles()

	fullPath := fmt.Sprintf("%s%s", baseUrl, *apiURL)
	method := strings.ToUpper(*apiMethod)

	var jsonBytes []byte
	if method != "GET" {
		jsonBytes, _ = prepareBody()
	}

	doHttpMethod(method, fullPath, jsonBytes, outputFile)

}

func doHttpMethod(method string, apiUrl string, data []byte, output string) {
	fmt.Printf("%s: %s\n", method, apiUrl)

	req, err := http.NewRequest(method, apiUrl, bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("#Error: creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("#Error: making request:", err)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("#Error: closing response body:", err)
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("#Error: reading response body:", err)
		return
	}

	var apiResponse ApiResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Println("Response ===================================== >>>")
		fmt.Printf("%s\n", string(body))
		fmt.Println("Response ===================================== <<<")
		fmt.Println("#Error: parsing JSON:", err)
		return
	}

	saveResponse(apiResponse, output)

	if apiResponse.Meta.Total > apiResponse.Meta.Page {
		nextPage := apiResponse.Meta.Page + 1
		fmt.Printf("fetching page %d of %d...\n", nextPage, apiResponse.Meta.Total)

		parsedParams, err := url.Parse(apiUrl)
		if err != nil {
			fmt.Println("#Error: parsing URL:", err)
			return
		}
		params := parsedParams.Query()
		params.Set("page", fmt.Sprintf("%d", nextPage))
		parsedParams.RawQuery = params.Encode()
		apiUrl = parsedParams.String()

		doHttpMethod("GET", apiUrl, nil, fmt.Sprintf("output_%d.csv", nextPage))
	}
}

func saveResponse(response ApiResponse, output string) {
	if !response.Success {
		fmt.Println("#Error: call was not successful")
		return
	}

	// Create CSV file
	csvFile, err := os.Create(output)
	if err != nil {
		fmt.Println("#Error: creating CSV file:", err)
		return
	}
	defer func(csvFile *os.File) {
		err := csvFile.Close()
		if err != nil {
			fmt.Println("#Error: closing CSV file:", err)
			return
		}
	}(csvFile)

	writer := csv.NewWriter(csvFile)

	// Write header
	if len(response.Data) == 0 {
		fmt.Println("#Error: no data to write to CSV")
		return
	}

	// Write header
	var header []string
	for key := range response.Data[0] {
		header = append(header, key)
	}
	err = writer.Write(header)
	if err != nil {
		fmt.Println("#Error: writing CSV header:", err)
		return
	}

	// Write data rows
	for _, row := range response.Data {
		var record []string
		for _, key := range header {
			value := fmt.Sprintf("%v", row[key])
			encoded, _ := ConvertToWindows1251(value)
			//if err != nil {
			//	fmt.Printf("failed to convert: %s\n", value)
			//}
			record = append(record, encoded)
		}
		err := writer.Write(record)
		if err != nil {
			fmt.Println("#Error: writing CSV record:", err)
			return
		}
	}

	writer.Flush()
	fmt.Printf("received %d records: %s\n", len(response.Data), output)
}

func prepareBody() ([]byte, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("opening CSV file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("#Error: closing CSV file:", err)
			return
		}
	}(file)

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV file: %w", err)
	}

	var jsonPayload []map[string]interface{}
	header := records[0]
	for _, row := range records[1:] {
		var record = make(map[string]interface{})
		for i, key := range header {
			field, err := ConvertToUTF8(row[i])
			if err != nil {
				fmt.Println("#Error: converting to utf-8:", err)
			}
			record[key] = field
		}
		jsonPayload = append(jsonPayload, record)
	}

	jsonBytes, err := json.Marshal(jsonPayload)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}
	fmt.Println("Body ===================================== >>>")
	fmt.Printf("%s\n", string(jsonBytes))
	fmt.Println("Body ===================================== <<<")

	return jsonBytes, nil
}

func ConvertToUTF8(win1251 string) (string, error) {
	decoder := charmap.Windows1251.NewDecoder()
	utf8Content, err := decoder.String(win1251)
	if err != nil {
		return "", err
	}
	return utf8Content, nil
}

func ConvertToWindows1251(utf8Str string) (string, error) {
	encoder := charmap.Windows1251.NewEncoder()
	win1251Content, err := encoder.String(utf8Str)
	if err != nil {
		return "", err
	}
	return win1251Content, nil
}

func RemoveFiles() {
	files, err := os.ReadDir("./")
	if err != nil {
		fmt.Println("reading directory:", err)
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			if strings.HasPrefix(file.Name(), "output") && strings.HasSuffix(file.Name(), ".csv") {
				err := os.Remove(file.Name())
				if err != nil {
					fmt.Printf("deleting file %s: %v\n", file.Name(), err)
				}
			}
		}
	}
}
