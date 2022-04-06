package main

import (
	cRand "crypto/rand"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"strconv"
)

const (
	SussiePath = "/etc/capatica/sussyphotos/"
	NormalPath = "/etc/capatica/safephotos/"
	Host       = "0.0.0.0"
	Port       = "4444"
)

var capaticasOpen []*Capatica
var normalImages []Image
var sussyImages []Image

func initialise() {
	// create required directories
	err := os.MkdirAll(SussiePath, 0755)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(NormalPath, 0755)
	if err != nil {
		panic(err)
	}
	fmt.Println("directories created, please add images before starting again")
	os.Exit(0)
}

func setup() {
	// read all files in sussy directory
	files, err := ioutil.ReadDir(SussiePath)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		// check if file is a jpg
		if f.Name()[len(f.Name())-3:] == "jpg" {
			// create image
			img := Image{
				RealName: f.Name(),
				Sus:      true,
			}
			// add image to sussyImages
			sussyImages = append(sussyImages, img)
		}
	}

	// read all files in normal directory
	files, err = ioutil.ReadDir(NormalPath)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		// check if file is a jpg
		if f.Name()[len(f.Name())-3:] == "jpg" {
			// create image
			img := Image{
				RealName: f.Name(),
				Sus:      false,
			}
			// add image to normalImages
			normalImages = append(normalImages, img)
		}
	}
}

func generateRequestID() string {
	// generate a random string
	b := make([]byte, 16)
	cRand.Read(b)
	return fmt.Sprintf("%x", b)
}

func createNewCapatica(capaticaChan chan *Capatica, errorChan chan error) {
	// seed random number generator from /dev/urandom
	// generate a request id
	requestID := generateRequestID()
	// choose 8 normal images and 1 sussy image
	var capaticaImages [9]*Image
	// choose a random image
	randomA, err := cRand.Int(cRand.Reader, big.NewInt(int64(len(normalImages))))
	if err != nil {
		errorChan <- err
	}
	// choose a random index to replace
	randomIndex, err := cRand.Int(cRand.Reader, big.NewInt(int64(len(normalImages))))
	if err != nil {
		errorChan <- err
	}
	for i := 0; i < 9; i++ {
		// if i is the random index, add the sussy image
		if int64(i) == randomIndex.Int64() {
			capaticaImages[i] = &sussyImages[randomA.Int64()]
		} else {
			// choose a random image
			randomB, err := cRand.Int(cRand.Reader, big.NewInt(int64(len(normalImages))))
			if err != nil {
				errorChan <- err
			}
			// add the image
			capaticaImages[i] = &normalImages[randomB.Int64()]
		}
	}
	fmt.Println("sussy image is " + sussyImages[randomA.Int64()].RealName)
	funny := fmt.Sprintf("at index %d", randomIndex.Int64())
	fmt.Println(funny)
	// create capatica
	capatica := Capatica{
		RequestID:  requestID,
		ImagesSent: capaticaImages,
	}
	// send capatica to capaticaChan
	capaticaChan <- &capatica

	// send error to errorChan
	errorChan <- nil
}

func errorPrinter(errorChan chan error) {
	err := <-errorChan
	if err != nil {
		fmt.Println(err)
	} else {
		return
	}
}

func httpServer(capaticaChan chan *Capatica, requestChan chan MainRoutineRequest) {
	r := mux.NewRouter()
	// get request to /genrequest to generate a new capatica
	r.HandleFunc("/genrequest", func(w http.ResponseWriter, r *http.Request) {
		// create error channel
		errorChan := make(chan error)
		// create capatica channel
		ourCapaticaChan := make(chan *Capatica)
		go createNewCapatica(ourCapaticaChan, errorChan)
		// wait for capatica to be created
		capatica := <-ourCapaticaChan
		// send capatica to capaticaChan
		capaticaChan <- capatica
		// wait for error
		errorPrinter(errorChan)
		// write response
		_, err := w.Write([]byte(capatica.RequestID))
		if err != nil {
			return
		}
	})

	// get request to /(requestID)/1-9 to get images
	r.HandleFunc("/{requestID}/{imageNumber}", func(w http.ResponseWriter, r *http.Request) {
		// get request id
		requestID := mux.Vars(r)["requestID"]
		fmt.Println(requestID)
		// get image number
		imageNumber := mux.Vars(r)["imageNumber"]
		// send request to requestChan for capatica
		returnChan := make(chan interface{})
		requestChan <- MainRoutineRequest{
			DemandType:        DemandGetCapatica,
			Data:              nil,
			AssociatedRequest: requestID,
			ResponseChan:      returnChan,
		}
		// wait for capatica
		capatica := <-returnChan
		// check type
		if capatica == nil {
			// capatica not found
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if reflect.TypeOf(capatica).String() == "main.Capatica" {
			// cast capatica
			capatica := capatica.(Capatica)
			// cast image number
			imageNumberInt, err := strconv.Atoi(imageNumber)
			if err != nil {
				fmt.Println(err)
				return
			}
			// check if image number is valid
			if imageNumberInt < 1 || imageNumberInt > 9 {
				fmt.Println("image number is not valid")
				return
			}
			// get image
			image := capatica.ImagesSent[imageNumberInt-1]
			// check if image is sussy
			if image.Sus {
				// send image to sussy
				fmt.Println("serving sussy image: " + image.RealName)
				http.ServeFile(w, r, SussiePath+image.RealName)
			} else {
				// send image to normal
				fmt.Println("serving normal image: " + image.RealName)
				http.ServeFile(w, r, NormalPath+image.RealName)
			}
		} else {
			// send error
			_, err := w.Write([]byte("error"))
			fmt.Println("type of capatica is not capatica, it is: " + reflect.TypeOf(capatica).String())
			if err != nil {
				fmt.Println(err)
			}
		}
	})

	// post request to /(requestID) with body of number 1-9 to verify capatica
	r.HandleFunc("/{requestID}/verify/{imageNumber}", func(w http.ResponseWriter, r *http.Request) {
		// get request id
		requestID := mux.Vars(r)["requestID"]
		// send request to requestChan for capatica
		returnChan := make(chan interface{})
		requestChan <- MainRoutineRequest{
			DemandType:        DemandGetCapatica,
			Data:              nil,
			AssociatedRequest: requestID,
			ResponseChan:      returnChan,
		}
		// wait for capatica
		capatica := <-returnChan
		// check type
		if capatica == nil {
			// capatica not found
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if reflect.TypeOf(capatica).String() == "main.Capatica" {
			// cast capatica
			capatica := capatica.(Capatica)
			// get image number
			imageNumber := mux.Vars(r)["imageNumber"]
			// cast image number
			imageNumberInt, err := strconv.Atoi(imageNumber)
			if err != nil {
				fmt.Println(err)
				return
			}
			// check if body is valid
			if imageNumberInt < 1 || imageNumberInt > 9 {
				fmt.Println("body is not valid")
				return
			}
			// check if number is the sussy image
			if capatica.ImagesSent[imageNumberInt-1].Sus {
				// send response
				_, err := w.Write([]byte("sussy"))
				if err != nil {
					return
				}
			} else {
				// send response
				_, err := w.Write([]byte("normal"))
				if err != nil {
					return
				}
			}
		} else {
			// send response
			_, err := w.Write([]byte("invalid"))
			if err != nil {
				return
			}
		}
	})

	// start server
	err := http.ListenAndServe(Host+":"+Port, r)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	// check if directories exist
	if _, err := os.Stat(SussiePath); os.IsNotExist(err) {
		fmt.Println("no sussy directory found, initializing")
		initialise()
	}
	if _, err := os.Stat(NormalPath); os.IsNotExist(err) {
		fmt.Println("no normal directory found, initializing")
		initialise()
	}

	// setup capatica images
	setup()

	// create capatica channel
	capaticaChan := make(chan *Capatica, 10000)

	// create mainroutinerequest channel
	requestChan := make(chan MainRoutineRequest, 1)

	// start http server
	go httpServer(capaticaChan, requestChan)

	for {
		if len(capaticaChan) > 0 {
			// add capatica to capaticasOpen
			tmp := <-capaticaChan
			capaticasOpen = append(capaticasOpen, tmp)
			fmt.Println("added capatica to capaticasOpen")
			fmt.Println("capaticasOpen length: " + strconv.Itoa(len(capaticasOpen)))
			fmt.Println(tmp)
		}
		if len(requestChan) > 0 {
			tmp := <-requestChan
			switch tmp.DemandType {
			case DemandVerifyCapatica:
				// check if capatica is in capaticasOpen
				resolved := false
				for i, capatica := range capaticasOpen {
					if capatica.RequestID == tmp.AssociatedRequest {
						// make sure tmp.Data is an int between 1 and 9 (the image that was clicked)
						if reflect.TypeOf(tmp.Data) == reflect.TypeOf(int(0)) {
							if tmp.Data.(int) > 0 && tmp.Data.(int) < 9 {
								// check if image is correct
								if capatica.ImagesSent[tmp.Data.(int)].Sus == true {
									// user picked sussy image
									tmp.ResponseChan <- true
								} else {
									// user picked bad image (they're an imposter)
									tmp.ResponseChan <- false
								}
							} else {
								// invalid image clicked
								fmt.Println("out of bounds")
								tmp.ResponseChan <- errors.New("out of bounds")
							}
						} else {
							// invalid data type
							fmt.Println("invalid data type")
							tmp.ResponseChan <- errors.New("invalid data type")

						}
						resolved = true
						capaticasOpen = append(capaticasOpen[:i], capaticasOpen[i+1:]...)
						break
					}
				}
				if !resolved {
					// capatica not found
					fmt.Println("capatica not found")
					tmp.ResponseChan <- errors.New("capatica not found")
				} else {
					// capatica found
					fmt.Println("capatica found")
					tmp.ResponseChan <- nil
				}
			case DemandGetCapatica:
				found := false
				for _, capatica := range capaticasOpen {
					if capatica.RequestID == tmp.AssociatedRequest {
						// send this one back over the return channel
						tmp.ResponseChan <- *capatica
						found = true
						break
					}
				}
				if !found {
					tmp.ResponseChan <- nil
				}
			}
		}
	}
}
