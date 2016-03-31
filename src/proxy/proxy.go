// Author: Devin Taylor and James Allingham

package main

import (
	"net"
	"strings"
	"fmt"
	"os"
	"io/ioutil"
	"lib"
	"strconv"
)

func main() {
	service := ":1236"

	listener, err := net.Listen("tcp", service)
	//packetConn, err := net.ListenPacket("udp", service)
	lib.CheckError(err)

	for {
	conn, err := listener.Accept()
	if err != nil {
		continue
	}
	go  handleClient(conn)
	}
}

func handleClient(conn net.Conn) {

	defer conn.Close()

	// get message of at maximum 512 bytes
	var buf [1024]byte
	for {
		// read input 
		_, err := conn.Read(buf[0:])
		// if there was an error exit
		if err != nil {
			return
		}
		// convert message to string and decompose it
	message := string(buf[0:])
	method, url, version, headers, body := lib.DecomposeRequest(message)
	// get the host ID
	var host string
	// find the hosts address
	for key, value := range headers {
		if(strings.ToUpper(key) == "HOST"){
			host = value
			break
		}
	}
	isInCache, lastModified, locationMap := checkInCache(url, strings.Split(host, ":")[0])

	if isInCache {
		headers = modifyHeaders(lastModified, headers)
		message = compileNewRequest(method, url, version, headers, body)
	}
	// strings.Split(host, ":")[0]

	// get the response message from the server
	serverResponse := handleServer(message, host)

	isUpdated, newResponse, newTime := getNewResponse(serverResponse, strings.Split(host, ":")[0], url)

	if isUpdated {
		destination := strings.Split(host, ":")[0]+url
		locationMap[destination] = newTime
		saveMap(locationMap, "../../cache/cache_map.txt")
	}
	// write the response message back to the client
	_, err = conn.Write(newResponse.ToBytes())
	lib.CheckError(err)
	}
}

func getNewResponse(serverResponse string, host string, url string) (bool, *lib.ResponseMessage, string) {
	version, code, status, headers, body := lib.DecomposeResponse(serverResponse)

	fmt.Println(code)

	if code == "304" {
		file, _ := os.Open("../../cache/"+host+url)
		defer file.Close()
		var response = lib.NewResponseMessage()
		response.Version = version
		response.HeaderLines = headers
		// compose 200
	    response.StatusCode = "200"
		response.Phrase = "OK"
		// read from file and convert to string
		b, _ := ioutil.ReadAll(file)
		html := string(b)
		response.EntityBody = html

		return false, response, ""
	} 

	StopIndex := strings.LastIndex(url, "/")
	newUrl := url[StopIndex:len(url)]

	host = host + url[0:StopIndex]

	if code == "200" {

		exists, _ := lib.FileExists("../../cache/"+host)
		if !exists {
			os.MkdirAll("../../cache/"+host, 0777)
		}

		ioutil.WriteFile("../../cache/"+host+newUrl, []byte(body), 0777)

		var response = lib.NewResponseMessage()
		response.Version = version
		response.HeaderLines = headers
		response.StatusCode = "200"
		response.Phrase = "OK"
		response.EntityBody = body
		newTime := headers["Last-Modified"]
		return true, response, newTime
	}

	var response = lib.NewResponseMessage()
	response.Version = version
	response.HeaderLines = headers
	response.StatusCode = code
	response.Phrase = status
	response.EntityBody = body

	return false, response, ""
}

func checkInCache(url string, host string) (bool, string, map[string]string) {
	locationMap := loadMap("../../cache/cache_map.txt")

	lastModified := locationMap[host+url]
	if lastModified != "" {
		return true, lastModified, locationMap
	}
	return false, "", locationMap
}

func modifyHeaders(lastModified string, headers map[string]string) map[string]string {
	headers["If-Modified-Since"] = lastModified

	return headers
}

func compileNewRequest(method string, url string, version string, headers map[string]string, body string) string {
	const sp = "\x20"
	const lf = "\x0a"
	const cr = "\x0d"
	requestString := method + sp
	requestString += url + sp
	requestString += version + cr + lf
	//add header lines
	for headerFieldName, value := range headers {
		requestString += headerFieldName + ":" + sp
		requestString += value + cr + lf
	}
	requestString += cr + lf
	requestString += body
	return requestString
}

func handleServer(relayRequest string, host string) string {
	// initiate connection
	conn, err := net.Dial("tcp", host)
	lib.CheckError(err)
	// write request information to the server
	_, err = conn.Write([]byte(relayRequest))
	lib.CheckError(err)
	// close the connection after this function executes
	defer conn.Close()

	var buf [65000]byte
	n, err := conn.Read(buf[0:])
	lib.CheckError(err)

	response := string(buf[0:n])
	version, code, status, headers, _ := lib.DecomposeResponse(response)

	headerTemp := lib.NewResponseMessage()
	headerTemp.Version = version
	headerTemp.StatusCode = code
	headerTemp.Phrase = status
	headerTemp.HeaderLines = headers
	headerTemp.EntityBody = ""
	headerSize := len(headerTemp.ToBytes())
	lengthDiff := 0

	contentLen, err := strconv.Atoi(headers["Content-Length"])
	if err == nil {
		lengthDiff = contentLen + headerSize - 65000
	} else {
		lengthDiff = -1
	}
	if strings.ToUpper(headers["Transfer-Encoding"]) == "CHUNKED" {

		for {
			// get message
			var buf [65000]byte
			// read input 
			n, err = conn.Read(buf[0:])
			lib.CheckError(err)
			response += string(buf[0:n])
			if strings.Contains(response, "\r\n0\r\n\r\n") || n == 0 {
					break
			}
		}
	} else {
		for lengthDiff > 0 {
			var buf [65000]byte
			// read input 
			n, err = conn.Read(buf[0:])
			lib.CheckError(err)
			response += string(buf[0:n])
			lengthDiff -= 65000
		}
		
	}

	return response
}