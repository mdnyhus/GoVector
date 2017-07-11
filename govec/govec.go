package govec

import (
	"bufio"
	"bytes"
	//"encoding/gob"
	"fmt"
	"github.com/DistributedClocks/GoVector/govec/vclock"
	"github.com/vmihailenco/msgpack"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
)

/*
   - All licences like other licenses ...

   How to Use This Library

   Step 1:
   Create a Global Variable and Initialize it using like this =

   Logger:= Initialize("MyProcess",ShouldYouSeeLoggingOnScreen,ShouldISendVectorClockonWire,Debug)

   Step 2:
   When Ever You Decide to Send any []byte , before sending call PrepareSend like this:
   SENDSLICE := PrepareSend("Message Description", YourPayload)
   and send the SENDSLICE instead of your Designated Payload

   Step 3:
   When Receiveing, AFTER you receive your message, pass the []byte into UnpackRecieve
   like this:

   UnpackReceive("Message Description", []ReceivedPayload, *RETURNSLICE)
   and use RETURNSLICE for further processing.
*/

const logToTerminal = false

//This is the Global Variable Struct that holds all the info needed to be maintained
type GoLog struct {

	//Local Process ID
	pid string

	//Local vector clock in bytes
	currentVC []byte

	//Flag to Printf the logs made by Local Program
	printonscreen bool

	//This bools Checks to send the VC Bundled with User Data on the Wire
	// if False, PrepareSend and UnpackReceive will simply forward their input
	//buffer to output and locally log event. If True, VC will be encoded into packet on wire
	VConWire bool

	// This bool checks whether we should send logging data to a vecbroker whenever
	// a message is logged.
	realtime bool

	//activates/deactivates printouts at each preparesend and unpackreceive
	debugmode bool

	logging bool

	//Logfile name
	logfile string

	// Publisher to enable sending messages to a vecbroker.
	publisher *GoPublisher

	logger *log.Logger

	mutex sync.RWMutex
}

//This is the data structure that is actually end on the wire
type ClockPayload struct {
	Pid         []byte
	Vcinbytes   []byte
	Payload 	interface{}
}

//Prints the Data Stuct as Bytes
func (d *ClockPayload) PrintDataBytes() {
	fmt.Printf("%x \n", d.Pid)
	fmt.Printf("%X \n", d.Vcinbytes)
	fmt.Printf("%X \n", d.Payload)
}

//Prints the Data Struct as a String
func (d *ClockPayload) PrintDataString() {
	fmt.Println("-----DATA START -----")
	s := string(d.Pid[:])
	fmt.Println(s)
	s = string(d.Vcinbytes[:])
	fmt.Println(s)
	fmt.Println("-----DATA END -----")
}

// TODO Pull request for this function
func (gv *GoLog) GetCurrentVCInBytes() []byte {
	return gv.currentVC
}

func (gv *GoLog) GetCurrentVC() vclock.VClock {
	vc, _ := vclock.FromBytes(gv.currentVC)
	return vc
}

func New() *GoLog {
	return &GoLog{}
}

func Initialize(processid string, logfilename string) *GoLog {
	/*This is the Start Up Function That should be called right at the start of
	  a program
	*/
	gv := New() //Simply returns a new struct
	gv.pid = processid

	if logToTerminal {
		gv.logger = log.New(os.Stdout, "[GoVector]:", log.Lshortfile)
	} else {
		var buf bytes.Buffer
		gv.logger = log.New(&buf, "[GoVector]:", log.Lshortfile)
	}

	//# These are bools that can be changed to change debuging nature of library
	gv.printonscreen = true //(ShouldYouSeeLoggingOnScreen)
	gv.debugmode = false    // (Debug)
	gv.EnableLogging()

	//we create a new Vector Clock with processname and 0 as the intial time
	vc1 := vclock.New()
	vc1.Tick(processid)

	//Vector Clock Stored in bytes
	//copy(gv.currentVC[:],vc1.Bytes())
	gv.currentVC = vc1.Bytes()

	if gv.debugmode == true {
		gv.logger.Println(" ###### Initialization ######")
		gv.logger.Print("VCLOCK IS :")
		vc1.PrintVC()
		gv.logger.Println(" ##### End of Initialization ")
	}

	//Starting File IO . If Log exists, Log Will be deleted and A New one will be created
	logname := logfilename + "-Log.txt"

	if _, err := os.Stat(logname); err == nil {
		//its exists... deleting old log
		gv.logger.Println(logname, "exists! ... Deleting ")
		os.Remove(logname)
	}

	// Create directory path to log if it doesn't exist.
	if err := os.MkdirAll(filepath.Dir(logname), 0750); err != nil {
		gv.logger.Println(err)
	}

	//Creating new Log
	file, err := os.Create(logname)
	if err != nil {
		gv.logger.Println(err)
	}
	file.Close()

	gv.logfile = logname
	//Log it
	ok := gv.LogThis("Initialization Complete", gv.pid, vc1.ReturnVCString())
	if ok == false {
		gv.logger.Println("Something went Wrong, Could not Log!")
	}

	return gv
}

func InitializeMultipleExecutions(processid string, logfilename string) *GoLog {
	/*This is the Start Up Function That should be called right at the start of
	  a program
	*/
	gv := New() //Simply returns a new struct
	gv.pid = processid

	if logToTerminal {
		gv.logger = log.New(os.Stdout, "[GoVector]:", log.Lshortfile)
	} else {
		var buf bytes.Buffer
		gv.logger = log.New(&buf, "[GoVector]:", log.Lshortfile)
	}

	//# These are bools that can be changed to change debuging nature of library
	gv.printonscreen = true //(ShouldYouSeeLoggingOnScreen)
	gv.debugmode = true     // (Debug)
	gv.EnableLogging()
	//we create a new Vector Clock with processname and 0 as the intial time
	vc1 := vclock.New()
	vc1.Tick(processid)

	//Vector Clock Stored in bytes
	//copy(gv.currentVC[:],vc1.Bytes())
	gv.currentVC = vc1.Bytes()

	if gv.debugmode == true {
		gv.logger.Println(" ###### Initialization ######")
		gv.logger.Print("VCLOCK IS :")
		vc1.PrintVC()
		gv.logger.Println(" ##### End of Initialization ")
	}

	//Starting File IO . If Log exists, it will find Last execution number and ++ it
	logname := logfilename + "-Log.txt"
	_, err := os.Stat(logname)
	gv.logfile = logname
	if err == nil {
		//its exists... deleting old log
		gv.logger.Println(logname, " exists! ...  Looking for Last Exectution... ")
		executionnumber := FindExecutionNumber(logname)
		executionnumber = executionnumber + 1
		gv.logger.Println("Execution Number is  ", executionnumber)
		executionstring := "=== Execution #" + strconv.Itoa(executionnumber) + "  ==="
		gv.LogThis(executionstring, "", "")
	} else {
		//Creating new Log
		file, err := os.Create(logname)
		if err != nil {
			gv.logger.Println(err.Error())
		}
		file.Close()

		//Mark Execution Number
		ok := gv.LogThis("=== Execution #1 ===", " ", " ")
		//Log it
		ok = gv.LogThis("Initialization Complete", gv.pid, vc1.ReturnVCString())
		if ok == false {
			gv.logger.Println("Something went Wrong, Could not Log!")
		}
	}
	return gv
}

func FindExecutionNumber(logname string) int {
	executionnumber := 0
	file, err := os.Open(logname)
	if err != nil {
		return 0
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for line, _, err := reader.ReadLine(); err != io.EOF; {
		if strings.Contains(string(line), " Execution #") {
			stemp := strings.Replace(string(line), "=== Execution #", "", -1)
			stemp = strings.Replace(stemp, " ===", "", -1)
			stemp = strings.Replace(stemp, " ", "", -1)
			currexecnumber, _ := strconv.Atoi(stemp)
			if currexecnumber > executionnumber {
				executionnumber = currexecnumber
			}
		}
		line, _, err = reader.ReadLine()
	}
	return executionnumber
}

//getCallingFunctionID returns the file name and line number of the
//program which called capture.go. This function is used to determine
//the calling function which did not receive a vector clock
func getCallingFunctionID() string {
	profiles := pprof.Profiles()
	block := profiles[1]
	var buf bytes.Buffer
	block.WriteTo(&buf, 1)
	//gv.logger.Printf("%s",buf)
	passedFrontOnStack := false
	re := regexp.MustCompile("([a-zA-Z0-9]+.go:[0-9]+)")
	ownFilename := regexp.MustCompile("capture.go") // hardcoded own filename
	matches := re.FindAllString(fmt.Sprintf("%s", buf), -1)
	for _, match := range matches {
		if passedFrontOnStack && !ownFilename.MatchString(match) {
			return match
		} else if ownFilename.MatchString(match) {
			passedFrontOnStack = true
		}
		//fmt.Printf("found %s\n", match)
	}
	//fmt.Printf("%s\n", buf)
	return ""
}

func (gv *GoLog) LogThis(Message string, ProcessID string, VCString string) bool {
	complete := true
	var buffer bytes.Buffer
	buffer.WriteString(ProcessID)
	buffer.WriteString(" ")
	buffer.WriteString(VCString)
	buffer.WriteString("\n")
	buffer.WriteString(Message)
	buffer.WriteString("\n")
	output := buffer.String()

	file, err := os.OpenFile(gv.logfile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		complete = false
	}
	defer file.Close()

	if _, err = file.WriteString(output); err != nil {
		complete = false
	}
	if gv.printonscreen == true {
		gv.logger.Println(output)
	}
	return complete

}

func (gv *GoLog) LogLocalEvent(Message string) bool {

	gv.mutex.Lock()
	//Converting Vector Clock from Bytes and Updating the gv clock
	vc, err := vclock.FromBytes(gv.currentVC)
	if err != nil {
		gv.logger.Println(err.Error())
	}
	_, found := vc.FindTicks(gv.pid)
	if found == false {
		gv.logger.Println("Couldnt find this process's id in its own vector clock!")
	}
	vc.Tick(gv.pid)
	gv.currentVC = vc.Bytes()

	var ok bool
	if gv.logging == true {
		ok = gv.LogThis(Message, gv.pid, vc.ReturnVCString())
		if gv.realtime == true {
			// Send local message to broker
			//BUG go publisher never worked // gv.publisher.PublishLocalMessage(Message, gv.pid, *vc)
		}
	}
	gv.mutex.Unlock()

	return ok
}

//This function is meant to be used immediately before sending.
//mesg will be logged along with the time of the send
//buf is encodeable data (structure or basic type)
//Returned is an encoded byte array with logging information
func (gv *GoLog) PrepareSend(mesg string, buf interface{}) []byte {
	/*
		This function is meant to be called before sending a packet. Usually,
		it should Update the Vector Clock for its own process, package with the clock
		using gob support and return the new byte array that should be sent onwards
		using the Send Command
	*/
	//Converting Vector Clock from Bytes and Updating the gv clock
	gv.mutex.Lock()
	vc, err := vclock.FromBytes(gv.currentVC)
	if err != nil {
		gv.logger.Println(err.Error())
	}
	_, found := vc.FindTicks(gv.pid)
	if found == false {
		gv.logger.Println("Couldnt find this process's id in its own vector clock!")
	}

	vc.Tick(gv.pid)
	gv.currentVC = vc.Bytes()

	if gv.debugmode == true {
		gv.logger.Println("Sending Message")
		gv.logger.Print("Current Vector Clock : ")
		vc.PrintVC()
	}

	var ok bool
	if gv.logging == true {
		ok = gv.LogThis(mesg, gv.pid, vc.ReturnVCString())
	}

	if ok == false {
		gv.logger.Println("Something went Wrong, Could not Log!")
	}

	// Create New Data Structure and add information: data to be transfered
	d := ClockPayload{}
	d.Pid = []byte(gv.pid)
	d.Vcinbytes = gv.currentVC
	d.Payload = buf

	//first layer of encoding (user data)
	encodedBytes, err := msgpack.Marshal(&d)
	if err != nil {
		gv.logger.Println(err.Error())
	}

	//return wrapperBuffer bytes which are wrapperEncoderoded as gob. Now these bytes can be sent off and
	// received on the other end!
	gv.mutex.Unlock()
	return encodedBytes
}

func (gv *GoLog) MergeIncomingClock(mesg string, e ClockPayload) {

	// First, tick the local clock
	vc, err := vclock.FromBytes(gv.currentVC)
	if gv.debugmode {
		gv.logger.Println("Received :")
		e.PrintDataString()
		gv.logger.Print("Received Vector Clock : ")
		vc.PrintVC()
	}

	_, found := vc.FindTicks(gv.pid)
	if !found {
		gv.logger.Println(fmt.Errorf("Couldnt find Local Process's ID in the Vector Clock. Could it be a stray message?"))
	}
	if err != nil {
		gv.logger.Println(err.Error())
	}
	vc.Tick(gv.pid)

	// Next, merge it with the new clock and update GV
	tmp := []byte(e.Vcinbytes[:])
	tempvc, err := vclock.FromBytes(tmp)

	if err != nil && gv.debugmode {
		gv.logger.Println(err.Error())
	}

	vc.Merge(tempvc)
	if gv.debugmode == true {
		gv.logger.Print("Now, Vector Clock is : ")
		vc.PrintVC()
	}
	gv.currentVC = vc.Bytes()

	// Log it
	var ok bool
	if gv.logging == true {
		ok = gv.LogThis(mesg, gv.pid, vc.ReturnVCString())
	}
	if ok == false {
		gv.logger.Println("Something went Wrong, Could not Log!")
	}

}

//UnpackReceive is used to unmarshall network data into local
//structures
//mesg will be logged along with the vector time the receive happened
//buf is the network data, previously packaged by PrepareSend
//unpack is a pointer to a structure, the same as was packed by
//PrepareSend
func (gv *GoLog) UnpackReceive(mesg string, buf []byte, unpack interface{}) {
	/*
		This function is meant to be called immediately after receiving a packet. It unpacks the data
		by the program, the vector clock. It updates vector clock and logs it. and returns the user data
	*/

	gv.mutex.Lock()

	e := ClockPayload{}
	e.Payload = unpack

	// Just use msgpack directly
	err := msgpack.Unmarshal(buf, &e)
	if err != nil {
		gv.logger.Println(err.Error())
	}

	// Increment and merge the incoming clock
	gv.MergeIncomingClock(mesg, e)
	gv.mutex.Unlock()

}

func (gv *GoLog) EnableLogging() {
	gv.logging = true
}

func (gv *GoLog) DisableLogging() {
	gv.logging = false
}
