package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// create a logger

var logger *log.Logger = log.New(os.Stdout, "", log.LstdFlags)

// A struct that gives information about a zone specified by a country, domain (link) and name

type Zone struct {
	Country string
	Domain  string
	Name    string
}

// Test Command
// await fetch("http://localhost:8989/append/", {
//     "credentials": "include",
//     "headers": {
//         "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0",
//         "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
//         "Accept-Language": "en-US",
//         "Upgrade-Insecure-Requests": "1",
//         "Sec-Fetch-Dest": "document",
//         "Sec-Fetch-Mode": "navigate",
//         "Sec-Fetch-Site": "none",
//         "Sec-Fetch-User": "?1"
//     },
//     "method": "POST",
//     "body":"{\"Country\":\"bg\",\"Domain\":\"data.bg\",\"Name\":\"Zone-1\"}",
//     "mode": "cors"
// });

// Zone string method

func (z *Zone) String() string {
	return fmt.Sprintf("%s (country: %s, domain: %s)", z.Name, z.Country, z.Domain)
}

// struct representing a map of zones with the key being the country

type Zones map[string]*Zone

// A string method for Zones

func (z Zones) String() string {
	var zones []string
	for country, zone := range z {
		zones = append(zones, fmt.Sprintf("%s (country: %s)", zone.Name, country))
	}
	return strings.Join(zones, "\n")
}

//A function that checks if a country is in a list of zones

func (z Zones) Check(country string) bool {
	_, ok := z[country]
	return ok
}

// Savings zones to a file
func (z Zones) Save(filename string) {
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	for _, zone := range z {
		fmt.Fprintf(file, "%s\t%s\t%s\n", zone.Country, zone.Domain, zone.Name)
	}
	file.Close()
}

// Loading zones from a file
func (z *Zones) Load(filename string) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), "\t")
		//append to the pointer of zones all the information about a zone from the line variable
		(*z)[line[0]] = new(Zone)
		(*z)[line[0]].Country = line[0]
		(*z)[line[0]].Domain = line[1]
		(*z)[line[0]].Name = line[2]
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

//Append a zone to the file if it doesn't already exist in the struct

func (z *Zones) Append(filename string, zone *Zone) {
	if _, ok := (*z)[zone.Country]; !ok {
		(*z)[zone.Country] = new(Zone)
		(*z)[zone.Country].Country = zone.Country
		(*z)[zone.Country].Domain = zone.Domain
		(*z)[zone.Country].Name = zone.Name
		logger.Print("Appending new zone: ", zone.Country, " ", zone.Domain, " ", zone.Name)
		go z.AppendRowToFile(filename, zone.Country, zone.Domain, zone.Name)

	}

}

func (*Zones) AppendRowToFile(filename string, country string, domain string, name string) {
	// open a file with append mode
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	//open a file and deffer for closing and apend the new zone to the file
	defer file.Close()

	file.Write([]byte(country + "\t" + domain + "\t" + name + "\n"))
}

func (z *Zones) Remove(filename string, country string) {
	if _, ok := (*z)[country]; ok {
		delete((*z), country)
		logger.Print("removing zone: ", country)
		go z.RemoveRowFromFile(filename, country)

	}

}

func (*Zones) RemoveRowFromFile(filename string, country string) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		panic(err)
	}
	file_content := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text_from_file := scanner.Text()
		val_to_check := strings.Split(text_from_file, "\t")[0]
		if val_to_check != country {
			file_content += text_from_file
		}
	}
	errClose := file.Close()
	if errClose != nil {
		panic(errClose)
	}

	fileTruncOpen, errOpenTrunc := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY, 0777)
	if errOpenTrunc != nil {
		panic(errOpenTrunc)
	}
	fileTruncOpen.WriteString(file_content)

	error_close := fileTruncOpen.Close()
	if error_close != nil {
		panic(error_close)
	}
}

// An enum in go
const (
	AppendZoneCommand = iota
	RemoveZoneCommand = iota
)

type ZoneCommand struct {
	command int8
	zone    *Zone
}

//Constructor for zone command

func NewZoneCommand(command int8, zone *Zone) *ZoneCommand {
	return &ZoneCommand{
		command: command,
		zone:    zone,
	}
}

func ZoneManager(zones *Zones, filename string, zoneChannel chan *ZoneCommand) {
	zones.Load(filename)
	defer zones.Save(filename)
	for {
		command := <-zoneChannel
		switch command.command {
		case AppendZoneCommand:
			go zones.Append(filename, command.zone)
		case RemoveZoneCommand:
			go zones.Remove(filename, (*command.zone).Country)
		}
	}
}

func messageHandler(zones *Zones) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(zones.String()))
	})
}

func appendHandler(zoneChannel chan *ZoneCommand) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var zone *Zone = new(Zone)
		err := decoder.Decode(zone)
		if err != nil {
			w.Write([]byte("Command was not successfull"))
			return
		}

		zoneChannel <- NewZoneCommand(AppendZoneCommand, zone)
		w.Write([]byte("Command recieved successfully"))
	})
}

// struct contating only a string

type RemoveZone struct {
	Country string
}

func removeHandler(zoneChannel chan *ZoneCommand) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var zone *Zone = new(Zone)
		var country RemoveZone
		err := decoder.Decode(&country)
		if err != nil {
			w.Write([]byte("Command was not successfull"))
			return
		}
		zone.Country = country.Country

		zoneChannel <- NewZoneCommand(RemoveZoneCommand, zone)
		w.Write([]byte("Command recieved successfully"))
	})
}

//     "body":"{\"Country\":\"bg\",\"Domain\":\"data.bg\",\"Name\":\"Zone-1\"}",

func sendHandler(zones *Zones) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		byte_data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Write([]byte("Command was not successfull"))
			return
		}

		bef, _, found := strings.Cut(string(byte_data), ":")
		if !found {
			w.Write([]byte("Command not formatted correctly"))
			return
		}
		bef, _, found = strings.Cut(string(bef), "/")
		if !found {
			w.Write([]byte("Command not formatted correctly"))
			return
		}
		zone, ok := (*zones)[bef]
		if !ok {
			w.Write([]byte("Country not found"))
		} else {
			// Create a HTTP post request
			req, err := http.NewRequest(http.MethodPost, zone.Domain, bytes.NewBuffer(byte_data))
			if err != nil {
				w.Write([]byte("req not successful"))
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(w, "client: error making http request: %s\n", err)
			}
			if res.StatusCode != 200 {
				w.Write([]byte("client: error status code not 200"))
			}
			byte_data, err = ioutil.ReadAll(res.Body)
			if err != nil {
				w.Write([]byte("client: error reading response body"))
			}
			logger.Print(string(byte_data[:]))
		}
	})
}

func main() {

	var zones Zones = make(Zones)
	//construct zones and load them from a file
	c := make(chan *ZoneCommand)
	go ZoneManager(&zones, "zones.txt", c)

	mux := http.NewServeMux()
	mux.Handle("/", messageHandler(&zones))
	mux.Handle("/append/", appendHandler(c))
	mux.Handle("/remove/", removeHandler(c))
	mux.Handle("/presend/", sendHandler(&zones))
	log.Print("Listening on :8989...")
	err := http.ListenAndServe(":8003", mux)
	log.Fatal(err)

}
