package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

// structure for a datapoint
type Datapoint struct {
	value     float32
	timestamp uint64
	metadata  string
}

// string function of datapoint
func (d *Datapoint) String() string {
	return fmt.Sprintf("%s:%f:%d", d.metadata, d.value, d.timestamp)
}

// chanel of datapoints
type DatapointChannel chan *Datapoint

// Datapoint list
type DatapointList []*Datapoint

// fast average of list if one value is added
func (d DatapointList) Average(avg float32, datapoint *Datapoint) float32 {
	val := float32(len(d))
	avg *= val
	avg += datapoint.value

	return avg / (val + 1)
}

// function to find the median of the list
func (d DatapointList) Median() float32 {
	if len(d) == 0 {
		return 0
	}

	if len(d)%2 == 0 {
		return (d[len(d)/2-1].value + d[len(d)/2].value) / 2
	} else {
		return d[len(d)/2].value
	}
}

// function to find the standard deviation of a list of datapoints
func (d DatapointList) StandardDeviation() float64 {
	if len(d) == 0 {
		return 0
	}

	var sum float32
	for _, d := range d {
		sum += d.value
	}

	return math.Sqrt(float64(sum) / float64(len(d)))
}

// save the list of datapoints to a file
func (d DatapointList) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		file.Close()
		return err
	}
	defer file.Close()
	arr := new(bytes.Buffer)
	err = binary.Write(arr, binary.LittleEndian, d[0].timestamp)
	if err != nil {
		return err
	}
	err = binary.Write(arr, binary.LittleEndian, d[0].value)
	if err != nil {
		return err
	}

	delta_compresion := uint16(d[1].timestamp - d[0].timestamp)
	err = binary.Write(arr, binary.LittleEndian, delta_compresion)
	if err != nil {
		return err
	}

	xor_compresed := math.Float32bits(d[1].value) ^ math.Float32bits(d[0].value)
	if xor_compresed == 0 {
		arr.WriteByte(0)
	} else {
		arr.WriteByte(1)
		err = binary.Write(arr, binary.LittleEndian, xor_compresed)
	}

	if err != nil {
		return err
	}

	for index, _ := range d[2:] {
		delta_delta_compresion := (d[index+2].timestamp - d[index+1].timestamp) - (d[index+1].timestamp - d[index+0].timestamp)
		if delta_delta_compresion == 0 {
			arr.WriteByte(0)
		} else if delta_delta_compresion < 256 {
			arr.WriteByte(1)
			err = binary.Write(arr, binary.LittleEndian, uint8(delta_compresion))
			if err != nil {
				return err
			}
		} else if delta_delta_compresion < 256*256 {
			arr.WriteByte(11)
			err = binary.Write(arr, binary.LittleEndian, uint16(delta_compresion))
			if err != nil {
				return err
			}
		} else {
			arr.WriteByte(111)
			err = binary.Write(arr, binary.LittleEndian, delta_compresion)
			if err != nil {
				return err
			}
		}
		//binary conversion from float to int
		xor_compresed := math.Float32bits(d[index+1].value) ^ math.Float32bits(d[index].value)
		if xor_compresed == 0 {
			arr.WriteByte(0)
		} else {
			arr.WriteByte(1)
			err = binary.Write(arr, binary.LittleEndian, xor_compresed)
		}

		if err != nil {
			return err
		}
	}
	file.Write(arr.Bytes())
	return nil
}

type Metadata struct {
	Start           uint64  `json:"start"`
	End             uint64  `json:"end"`
	Median          float32 `json:"median"`
	Std             float64 `json:"std"`
	Avg             float32 `json:"avg"`
	Metadata        string  `json:"metadata"`
	DatapointsCount uint32  `json:"datapoints-count"`
}

func saveMetadata(from uint64, to uint64, median float32, std float64, avg float32, folder string, metadata string, datapoints_count uint32) error {
	file, err := os.Create(folder + "/metadata.json")
	defer file.Close()
	if err != nil {
		return err
	}
	ToSaveData := Metadata{}
	ToSaveData.Start = from
	ToSaveData.End = to
	ToSaveData.Median = median
	ToSaveData.Std = std
	ToSaveData.Avg = avg
	ToSaveData.Metadata = metadata
	ToSaveData.DatapointsCount = datapoints_count
	res, err := json.Marshal(&ToSaveData)
	if err != nil {
		return err
	}
	file.Write(res)
	return nil
}

// function to manage the channel of datapoints
func DatapointManager(channel DatapointChannel) {
	var avg float32 = 0
	var std float64 = 0
	var median float32 = 0
	var begin_timestamp uint64 = math.MaxUint64
	var end_timestamp uint64 = 0
	time_delta := uint64(60)
	//create datapoint list
	datapointList := make(DatapointList, 0)
	for {
		//get datapoint from channel
		datapoint := <-channel

		//extract the time from a datapoint
		timestamp := datapoint.timestamp
		//fmt.Println(timestamp)
		begin_timestamp = uint64(math.Min(float64(begin_timestamp), float64(timestamp)))
		end_timestamp = uint64(math.Max(float64(end_timestamp), float64(timestamp)))
		avg = datapointList.Average(avg, datapoint)
		//add datapoint to list
		datapointList = append(datapointList, datapoint)
		if (uint64(timestamp)-begin_timestamp)/60 >= time_delta {
			folder_name := "data/" + strconv.FormatInt(int64(begin_timestamp), 10) + "-" + strconv.FormatInt(int64(end_timestamp), 10)
			os.Mkdir(folder_name, 0755)

			std = datapointList.StandardDeviation()
			median = datapointList.Median()
			std += 1
			err := saveMetadata(begin_timestamp, end_timestamp, median, std, avg, folder_name, datapointList[0].metadata, uint32(len(datapointList)))
			if err != nil {
				fmt.Println(err)
			}
			//fmt.Println("Average value: ", avg)
			//fmt.Println("Standard deviation: ", std)
			//fmt.Println("begin: ", begin_timestamp, "\nend: ", end_timestamp)
			datapointList2 := datapointList
			go datapointList2.Save(folder_name + "/DatapointList_From_" + strconv.FormatInt(int64(begin_timestamp), 10) + "_to_" + strconv.FormatInt(int64(end_timestamp), 10))
			begin_timestamp = math.MaxUint64
			end_timestamp = 0
			datapointList = make(DatapointList, 0)
		}
	}

}

func DatapointHandlerAdd(channel DatapointChannel) http.HandlerFunc {
	// getDatapoint function to extract datapoint from request
	getDatapoint := (func(r *http.Request) (*Datapoint, error) {
		// get datapoint
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		datapoint := new(Datapoint)
		status := 0

		for i := range data {
			if data[i] == 58 {
				if status == 0 {
					datapoint.metadata = string(data[0:i])
					status = i
				} else {
					value, err := strconv.ParseFloat(string(data[status+1:i]), 32)
					if err != nil {
						return nil, err
					}
					datapoint.value = float32(value)
					status = i
					break
				}
			}
		}
		timestamp, err := strconv.ParseUint(string(data[status+1:]), 10, 64)
		//print timestamp
		if err != nil {
			return nil, err
		}
		datapoint.timestamp = uint64(timestamp)
		return datapoint, nil
	})
	// http handler to get datapoint
	return func(w http.ResponseWriter, r *http.Request) {
		// get datapoint
		datapoint, err := getDatapoint(r)
		fmt.Println(datapoint)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		channel <- datapoint
		//print datapoint
		//fmt.Printf("%s\n", datapoint.String())
	}
}

func CloseAfter15Seconds() {
	time.Sleep(15 * time.Second)
	pprof.StopCPUProfile()
}

func getTime(r *http.Request) (uint64, uint64, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, 0, err
	}
	var from uint64 = 0
	var to uint64 = 0
	index := 0
	for i := range data {
		if data[i] == 58 {
			from, err = strconv.ParseUint(string(data[0:i]), 10, 64)

			if err != nil {
				return 0, 0, err
			}
			index = i
			break
		}
	}
	to, err = strconv.ParseUint(string(data[index+1:]), 10, 64)

	if err != nil {
		return 0, 0, err
	}
	return from, to, nil

}

func DatapointHandlerGetAverage(channel DatapointChannel) http.HandlerFunc {

	// http handler to get datapoint
	return func(w http.ResponseWriter, r *http.Request) {

		from, to, err := getTime(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		folders, err := os.ReadDir("data")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		positive := make([]string, 0)
		var beginPartial string = ""
		var endPartial string = ""
		for _, f := range folders {
			lister := strings.Split(f.Name(), "-")
			fromRep, toRep := lister[0], lister[1]
			fromRepNum, err := strconv.ParseUint(fromRep, 10, 64)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			toRepNum, err := strconv.ParseUint(toRep, 10, 64)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if fromRepNum >= from && toRepNum <= to {
				positive = append(positive, f.Name())
			} else if fromRepNum < to && toRepNum > to {
				endPartial = f.Name()
			} else if toRepNum > from && fromRepNum < from {
				beginPartial = f.Name()
			}
		}
		fmt.Println(beginPartial)
		val := 0.00
		count := 0.0
		fmt.Println(len(positive))
		for _, name := range positive {
			//load json
			file, err := os.Open("data/" + name + "/metadata.json")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer file.Close()

			var metadata Metadata
			err = json.NewDecoder(file).Decode(&metadata)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			val += float64(metadata.Avg * float32(metadata.DatapointsCount))
			count += float64(metadata.DatapointsCount)
		}
		fmt.Println(endPartial)
		fmt.Fprint(w, "{\"value\":", val/count, "}")
	}
}

// main function that uses the getDatapointHandler function
func main() {
	//creating a datapoint channel

	//pprof.StartCPUProfile(os.Stdout)
	//go CloseAfter15Seconds()
	datapoint_channel := make(DatapointChannel)

	go DatapointManager(datapoint_channel)
	// start HTTP server
	http.HandleFunc("/datapoint-post", DatapointHandlerAdd(datapoint_channel))
	http.HandleFunc("/datapoint-get-average", DatapointHandlerGetAverage(datapoint_channel))
	http.ListenAndServe(":8001", nil)
}
