# API Call
Perform simple API calls and save the results into CSV files.

The program accepts command-line parameters that specify the method name and resource path: `-url` and `-method`.
Config file `config.yml` must contain a base URL for API calls.
If a method requires a JSON-formatted body, place an `input.csv` file adjacent to the application binary. Before executing the HTTP method, the CSV file will be converted into a JSON payload.
Supports authentication with a bearer token.

The data received from the API call is saved into an `output.csv` file.

Logs and errors are recorded in an `errors.log` file.

Cyrillic values are converted from Windows-1251 to UTF-8 encoding before sending. Received data is converted vice versa.

## Usage
Place the `config.yml` file in the same directory as the application binary, or use parameter `-config` to specify the path to the config file.
In the config file, specify the base URL for API calls.
```yml
---

base_url: https://my-test.site/api/v1
input_path: c:\work_dir\
output_path: c:\work_dir\
bearer_token: my_token
```
It's possible to use parameter `-path` to specify the working directory. In that case the config parameters `input_path` and `output_path` will be ignored.
Example command line with the path and config parameters:
```bash
call.exe -config=c:\api\config.yml -path=c:\work_dir\ -url=/resource -method=GET
```
### GET request
To make a GET request on url https://my-test.site/api/v1/resourse, run the application with the following command:
```bash
call.exe -url=/resource -method=GET
```
### POST request
To make POST requests, create an `input.csv` file in the input directory, `input_path` parameter of config file or `-path` in command line. The first row must contain the JSON keys. The following rows must contain the JSON values.
```csv
key1,key2,key3
value1,value2,value3
value4,value5,value6
```
This example will create the following JSON payload:
```json
[
    {
        "key1": "value1",
        "key2": "value2",
        "key3": "value3"
    },
    {
        "key1": "value4",
        "key2": "value5",
        "key3": "value6"
    }
]
```
Example of a POST request, body is taken from `input.csv` file:
```bash
call.exe -url=/resource -method=POST
```
To send a single JSON object, create an `object.csv` file with two rows, keys and values.
### POST file as boundary
To send a file as a boundary, use parameter `-boundary` with the file name.
```bash
call.exe -url=/resource -method=POST -boundary=[fileName]
```
`fileName` is the name of the file to be sent as a boundary, it must be placed in the input directory, `input_path` parameter in config file or `-path` in command line.
### Help
To display the help message, run the application with the following command:
```bash
call.exe -help
```