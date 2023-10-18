# API Call
Perform simple API calls and save the results into CSV files.

The program accepts command-line parameters that specify the method name and resource path: "-url", "-method".
Config file "config.yml" must contain a base URL for API calls.
If a method requires a JSON-formatted body, place an "input.csv" file adjacent to the application binary. Before executing the HTTP method, the CSV file will be converted into a JSON payload.

The data received from the API call is saved into an "output.csv" file.

Logs and errors are recorded in an "errors.log" file.

Cyrillic values are converted from Windows-1251 to UTF-8 encoding before sending. Received data is converted vice versa.
