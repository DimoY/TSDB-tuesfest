package main

import (
	"bufio"
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

// A struct that gives information about a leaf specified by a country, domain (link) and name

type Leaf struct {
	Tag    string
	Domain string
	Name   string
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
//     "body":"{\"Country\":\"bg\",\"Domain\":\"data.bg\",\"Name\":\"leaf-1\"}",
//     "mode": "cors"
// });

// leaf string method

func (z *Leaf) String() string {
	return fmt.Sprintf("%s (country: %s, domain: %s)", z.Name, z.Tag, z.Domain)
}

// struct representing a map of leafs with the key being the country

type Leafs map[string]*Leaf

// A string method for leafs

func (z Leafs) String() string {
	var leafs []string
	for tag, leaf := range z {
		leafs = append(leafs, fmt.Sprintf("%s (country: %s)", leaf.Name, tag))
	}
	return strings.Join(leafs, "\n")
}

//A function that checks if a country is in a list of leafs

func (z Leafs) Check(country string) bool {
	_, ok := z[country]
	return ok
}

// Savings leafs to a file
func (z Leafs) Save(filename string) {
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	for _, leaf := range z {
		fmt.Fprintf(file, "%s\t%s\t%s\n", leaf.Tag, leaf.Domain, leaf.Name)
	}
	file.Close()
}

// Loading leafs from a file
func (z *Leafs) Load(filename string) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), "\t")
		//append to the pointer of leafs all the information about a leaf from the line variable
		(*z)[line[0]] = new(Leaf)
		(*z)[line[0]].Tag = line[0]
		(*z)[line[0]].Domain = line[1]
		(*z)[line[0]].Name = line[2]
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

//Append a leaf to the file if it doesn't already exist in the struct

func (z *Leafs) Append(filename string, leaf *Leaf) {
	if _, ok := (*z)[leaf.Tag]; !ok {
		(*z)[leaf.Tag] = new(Leaf)
		(*z)[leaf.Tag].Tag = leaf.Tag
		(*z)[leaf.Tag].Domain = leaf.Domain
		(*z)[leaf.Tag].Name = leaf.Name
		logger.Print("Appending new leaf: ", leaf.Tag, " ", leaf.Domain, " ", leaf.Name)
		go z.AppendRowToFile(filename, leaf.Tag, leaf.Domain, leaf.Name)

	}

}

func (*Leafs) AppendRowToFile(filename string, country string, domain string, name string) {
	// open a file with append mode
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	//open a file and deffer for closing and apend the new leaf to the file
	defer file.Close()

	file.Write([]byte(country + "\t" + domain + "\t" + name + "\n"))
}

func (z *Leafs) Remove(filename string, country string) {
	if _, ok := (*z)[country]; ok {
		delete((*z), country)
		logger.Print("removing leaf: ", country)
		go z.RemoveRowFromFile(filename, country)

	}

}

func (*Leafs) RemoveRowFromFile(filename string, country string) {
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
	AppendleafCommand = iota
	RemoveleafCommand = iota
)

type leafCommand struct {
	command int8
	leaf    *Leaf
}

//Constructor for leaf command

func NewleafCommand(command int8, leaf *Leaf) *leafCommand {
	return &leafCommand{
		command: command,
		leaf:    leaf,
	}
}

func leafManager(leafs *Leafs, filename string, leafChannel chan *leafCommand) {
	leafs.Load(filename)
	defer leafs.Save(filename)
	for {
		command := <-leafChannel
		switch command.command {
		case AppendleafCommand:
			go leafs.Append(filename, command.leaf)
		case RemoveleafCommand:
			go leafs.Remove(filename, (*command.leaf).Tag)
		}
	}
}

func messageHandler(leafs *Leafs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(leafs.String()))
	})
}

func appendHandler(leafChannel chan *leafCommand) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var leaf *Leaf = new(Leaf)
		err := decoder.Decode(leaf)
		if err != nil {
			w.Write([]byte("Command was not successfull"))
			return
		}

		leafChannel <- NewleafCommand(AppendleafCommand, leaf)
		w.Write([]byte("Command recieved successfully"))
	})
}

// struct contating only a string

type RemoveLeaf struct {
	Country string
}

func removeHandler(leafChannel chan *leafCommand) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var leaf *Leaf = new(Leaf)
		var country RemoveLeaf
		err := decoder.Decode(&country)
		if err != nil {
			w.Write([]byte("Command was not successfull"))
			return
		}
		leaf.Tag = country.Country

		leafChannel <- NewleafCommand(RemoveleafCommand, leaf)
		w.Write([]byte("Command recieved successfully"))
	})
}

func sendHandler(leafs *Leafs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		byte_data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Write([]byte("LeafTag was not successfull"))
			return
		}

		bef, after, found := strings.Cut(string(byte_data), ":")
		if !found {
			w.Write([]byte("LeafTag not formatted correctly"))
			return
		}
		bef, _, found = strings.Cut(after, ":")
		if !found {
			w.Write([]byte("LeafTag not formatted correctly"))
			return
		}
		_, ok := (*leafs)[bef]
		if !ok {
			w.Write([]byte("LeafTag not found"))
		} else {
			w.Write([]byte("LeafTag found request resend"))
			logger.Print(string(byte_data[:]))
		}
	})
}

func main() {

	var leafs Leafs = make(Leafs)
	//construct leafs and load them from a file
	c := make(chan *leafCommand)
	go leafManager(&leafs, "leafs.txt", c)

	mux := http.NewServeMux()
	mux.Handle("/", messageHandler(&leafs))
	mux.Handle("/append/", appendHandler(c))
	mux.Handle("/remove/", removeHandler(c))
	mux.Handle("/presend/", sendHandler(&leafs))
	log.Print("Listening on :8989...")
	err := http.ListenAndServe(":8989", mux)
	log.Fatal(err)

}
