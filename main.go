package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

const (
	outputFile = "output.csv"
	inputFile  = "input.csv"
	objectFile = "object.csv"
)

type ApiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	//Errors  []string                 `json:"errors"`
	Data []map[string]interface{} `json:"data"`
	Meta PageData                 `json:"meta"`
}

type PageData struct {
	Page  int `json:"page"`
	Total int `json:"totalPage"`
}

type Api struct {
	url        string
	inputPath  string
	outputPath string
	token      string
	debug      bool
}

func main() {
	fmt.Println("...Starting Api Caller v1.0.3 (c) 2025 dev@programmer.com.ua")
	now := time.Now()

	configPath := flag.String("conf", "config.yml", "path to config file")
	apiURL := flag.String("url", "", "API resource URL to fetch data from")
	apiMethod := flag.String("method", "GET", "HTTP method (GET, POST, etc.)")
	workPath := flag.String("path", "", "working directory")
	boundary := flag.String("boundary", "", "File name to be send using boundary")
	debug := flag.Bool("debug", false, "enable debug mode")
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

	api := Api{
		url:        fmt.Sprintf("%s%s", baseUrl, *apiURL),
		inputPath:  conf.InputPath,
		outputPath: conf.OutputPath,
		token:      conf.BearerToken,
	}
	if workPath != nil && *workPath != "" {
		api.inputPath = *workPath
		api.outputPath = *workPath
	}
	if *debug {
		fmt.Println("Debug mode is ON")
		fmt.Println("Config file:", *configPath)
		fmt.Println("API URL:", *apiURL)
		fmt.Println("API Method:", *apiMethod)
		fmt.Println("Working directory:", workPath)
		fmt.Println("Boundary:", *boundary)
		api.debug = true
	}

	logFile := fmt.Sprintf("%serrors.log", api.outputPath)
	_ = os.Remove(logFile)
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("opening or creating log file: %v\n", err)
		return
	}
	defer func(file *os.File) {
		fmt.Printf("Finished in %s\n", time.Since(now))
		err = file.Close()
		if err != nil {
			fmt.Println("closing log file:", err)
			return
		}
	}(file)
	os.Stdout = file

	api.removeFiles()

	method := strings.ToUpper(*apiMethod)

	if boundary != nil && *boundary != "" && method == "POST" {
		api.doMultipartPost(*boundary)
		return
	}

	var jsonBytes []byte
	if method != "GET" {
		jsonBytes, err = prepareBody(api.inputPath)
		if err != nil {
			fmt.Println("#Error: preparing body:", err)
			return
		}
	}

	api.doHttpMethod(method, jsonBytes, outputFile)

}

func (a *Api) doHttpMethod(method string, data []byte, output string) {
	fmt.Printf("%s: %s\n", method, a.url)

	req, err := http.NewRequest(method, a.url, bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("#Error: creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("#Error: making request:", err)
		return
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
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

	if a.debug {
		fmt.Println("Response ===================================== >>>")
		fmt.Printf("%s\n", string(body))
		fmt.Println("Response ===================================== <<<")
	}

	var apiResponse ApiResponse
	//err = json.Unmarshal(body, &apiResponse)
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	err = dec.Decode(&apiResponse)
	if err != nil {
		fmt.Println("#Error: parsing JSON:", err)
		return
	}

	if !apiResponse.Success {
		if apiResponse.Message != "" {
			fmt.Println("#Error: ", apiResponse.Message)
		}
		//if len(apiResponse.Errors) > 0 {
		//	fmt.Println("#Error: ", apiResponse.Errors)
		//}
		return
	}

	a.saveResponse(apiResponse, output)

	if apiResponse.Meta.Total > apiResponse.Meta.Page {
		nextPage := apiResponse.Meta.Page + 1
		fmt.Printf("fetching page %d of %d...\n", nextPage, apiResponse.Meta.Total)

		parsedParams, err := url.Parse(a.url)
		if err != nil {
			fmt.Println("#Error: parsing URL:", err)
			return
		}
		params := parsedParams.Query()
		params.Set("page", fmt.Sprintf("%d", nextPage))
		parsedParams.RawQuery = params.Encode()
		a.url = parsedParams.String()

		a.doHttpMethod("GET", nil, fmt.Sprintf("output_%d.csv", nextPage))
	}
}

func (a *Api) saveResponse(response ApiResponse, output string) {
	if !response.Success {
		fmt.Println("#Error: call was not successful")
		return
	}

	// Create CSV file
	csvFile, err := os.Create(fmt.Sprintf("%s%s", a.outputPath, output))
	if err != nil {
		fmt.Println("#Error: creating file:", err)
		return
	}
	defer func(csvFile *os.File) {
		err = csvFile.Close()
		if err != nil {
			fmt.Println("#Error: closing file:", err)
			return
		}
	}(csvFile)

	writer := csv.NewWriter(csvFile)

	// Write header
	if len(response.Data) == 0 {
		fmt.Println("#Warn: no data to write")
		return
	}

	// Write header
	var header []string
	for key := range response.Data[0] {
		header = append(header, key)
	}
	err = writer.Write(header)
	if err != nil {
		fmt.Println("#Error: writing header:", err)
		return
	}

	// Write data rows
	for _, row := range response.Data {
		var record []string
		for _, key := range header {
			value := fmt.Sprintf("%v", row[key])
			value = strings.ReplaceAll(value, "\n", " ")
			value = strings.ReplaceAll(value, "\r", "")
			encoded, e := ConvertToWindows1251(value)
			if a.debug && e != nil {
				fmt.Printf("#Error: converting string: %s\n", e)
				fmt.Printf("#Error: failed to convert: %s\n", value)
			}
			record = append(record, encoded)
		}
		err = writer.Write(record)
		if err != nil {
			fmt.Println("#Error: writing record:", err)
			return
		}
	}

	writer.Flush()
	fmt.Printf("received %d records: %s\n", len(response.Data), output)
}

func prepareBody(path string) ([]byte, error) {

	singleFile, err := readFileContent(path, objectFile)
	if err == nil {
		if len(singleFile) > 0 {
			obj := singleFile[0]
			return getJsonBytes(obj)
		}
		return nil, fmt.Errorf("empty object data file")
	}

	singleFile, err = readFileContent(path, inputFile)
	if err == nil {
		return getJsonBytes(singleFile)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %s: %s", path, err)
	}

	result := make(map[string][]map[string]interface{})

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "input_") && strings.HasSuffix(file.Name(), ".csv") {

			jsonPayload, err := readFileContent(path, file.Name())
			if err != nil {
				return nil, fmt.Errorf("reading file content: %s: %w", file.Name(), err)
			}

			keyName := strings.TrimPrefix(file.Name(), "input_")
			keyName = strings.TrimSuffix(keyName, ".csv")

			result[keyName] = jsonPayload
		}
	}

	return getJsonBytes(result)
}

func readFileContent(path, fileName string) ([]map[string]interface{}, error) {
	file, err := os.Open(fmt.Sprintf("%s%s", path, fileName))
	if err != nil {
		return nil, fmt.Errorf("opening file: %s: %s", fileName, err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("#Error: closing file:", err)
			return
		}
	}(file)

	fmt.Println("Reading file:", fileName)

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
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

	return jsonPayload, nil
}

func getJsonBytes(v any) ([]byte, error) {
	jsonBytes, err := json.Marshal(v)
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
	enc := encoding.ReplaceUnsupported(charmap.Windows1251.NewEncoder())
	win1251Content, err := enc.String(utf8Str)
	if err != nil {
		return "", err
	}

	// replace all '?' (replacement) with space
	win1251Content = strings.ReplaceAll(win1251Content, "?", " ")

	// collapse multiple spaces into one
	win1251Content = strings.Join(strings.FieldsFunc(win1251Content, unicode.IsSpace), " ")

	return win1251Content, nil
}

func (a *Api) removeFiles() {
	files, err := os.ReadDir(a.outputPath)
	if err != nil {
		fmt.Println("reading directory:", err)
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			if strings.HasPrefix(file.Name(), "output") && strings.HasSuffix(file.Name(), ".csv") {
				err := os.Remove(fmt.Sprintf("%s%s", a.outputPath, file.Name()))
				if err != nil {
					fmt.Printf("deleting file %s: %v\n", file.Name(), err)
				}
			}
		}
	}
}

func (a *Api) doMultipartPost(boundary string) {
	fmt.Printf("POST: %s\n", a.url)

	file, err := os.Open(fmt.Sprintf("%s%s", a.inputPath, boundary))
	if err != nil {
		fmt.Println("#Error: opening file:", err)
		return
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			fmt.Println("#Error: closing file:", err)
			return
		}
	}(file)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", boundary)
	if err != nil {
		fmt.Println("#Error: creating form file:", err)
		return
	}

	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Println("#Error: copying file to form file:", err)
		return
	}

	err = writer.Close()
	if err != nil {
		fmt.Println("#Error: closing writer:", err)
		return
	}

	fmt.Println("Body ===================================== >>>")
	fmt.Printf("%s\n", body)
	fmt.Println("Body ===================================== <<<")

	req, err := http.NewRequest("POST", a.url, body)
	if err != nil {
		fmt.Println("#Error: creating request:", err)
		return
	}
	content := writer.FormDataContentType()
	fmt.Println("Content-Type:", content)
	req.Header.Set("Content-Type", content)

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

	if resp.StatusCode > 299 {
		fmt.Printf("#Error: response status %s\n", resp.Status)
	}

	//response, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	fmt.Println("#Error: reading response body:", err)
	//	return
	//}
	//
	//fmt.Println("Response ===================================== >>>")
	//fmt.Printf("%s\n", string(response))
	//fmt.Println("Response ===================================== <<<")
}

// DecodeJSON parses a JSON-encoded byte slice into a generic interface and validates that the top-level object is a map.
// It converts JSON numbers to int64 or float64 where applicable for numeric accuracy. Returns the parsed object or an error.
func DecodeJSON(body []byte) (map[string]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}

	obj, ok := normalizeNumbers(v).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected top-level JSON object, got %T", v)
	}
	return obj, nil
}

// normalizeNumbers converts json.Number types to int64 or float64 where applicable, preserving the structure of the input.
func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[k] = normalizeNumbers(val)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, val := range x {
			s[i] = normalizeNumbers(val)
		}
		return s
	case json.Number:
		// Сначала пытаемся как целое
		if i, err := x.Int64(); err == nil {
			return i
		}
		// Иначе как float64 (учти возможную потерю точности у очень больших чисел)
		if f, err := x.Float64(); err == nil {
			return f
		}
		// Фолбэк — оставить строкой
		return x.String()
	default:
		return v
	}
}
