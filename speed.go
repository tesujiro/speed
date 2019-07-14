package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var DEBUG bool = false

type ByteSize float64

const (
	_           = iota             // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota) // 1 << (10*1)
	MB                             // 1 << (10*2)
	GB                             // 1 << (10*3)
	TB                             // 1 << (10*4)
	PB                             // 1 << (10*5)
	EB                             // 1 << (10*6)
	ZB                             // 1 << (10*7)
	YB                             // 1 << (10*8)
)

func BinaryPrefixDict() func(string) ByteSize {
	dict := map[string]ByteSize{
		"K": KB, "M": MB, "G": GB, "T": TB, "P": PB, "E": EB, "Z": EB, "Y": YB,
	}
	for k, v := range dict {
		dict[k+"i"] = v
	}
	dict[""] = 1
	return func(key string) ByteSize {
		return dict[key]
	}
}

// inBynaryPrefix(1024) => (1.0,"K")
func inBynaryPrefix(d int) (float64, string) {
	plist := []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	dict := BinaryPrefixDict()
	for i, p := range plist {
		if d < int(dict(p)) {
			return float64(d) / float64(dict(plist[i-1])), plist[i-1]
		}
	}
	return float64(d), ""
}

// parseBynaryPrefix("1KB") => 1024
func parseBynaryPrefix(bp string) (int, error) {
	var size int
	bp_regex := regexp.MustCompile(`^([\d]+)([[KMGTPEZY]i?]?)?B?$`)
	result := bp_regex.FindAllStringSubmatch(bp, -1)
	if len(result) == 0 {
		return 0, fmt.Errorf("Parse String error: %v", bp)
	}
	if i, err := strconv.Atoi(result[0][1]); err != nil {
		return 0, fmt.Errorf("Convert string to number error: %v", result[0][1])
	} else {
		if len(result[0]) > 2 {
			unit := result[0][2]
			size = i * int(BinaryPrefixDict()(unit))
		} else {
			size = i
		}
	}
	return size, nil
}

func dprintf(format string, a ...interface{}) {
	if DEBUG {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getWidth() uint {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return uint(ws.Col)
}

func openfile(filename string) (*os.File, os.FileInfo, error) {
	filePath, err := filepath.Abs(filename)
	if err != nil {
		return nil, nil, err
	}

	fileinfo, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	return file, fileinfo, nil
}

type option struct {
	speed     int
	tty       string
	silent    bool
	graph     bool
	filename  string
	show_data bool
}

func getOption() *option {
	var bw *string = flag.String("b", "", "Bytes Per Sec.")
	var tty *string = flag.String("t", "/dev/tty", "tty device name. default: tty")
	var silent *bool = flag.Bool("s", false, "Silent Mode")
	var graph *bool = flag.Bool("g", false, "Graphic Mode")
	var data *bool = flag.Bool("a", false, "Show All Data Mode")
	var debug *bool = flag.Bool("d", false, "Debug Mode")
	flag.Parse()

	// Parse "bandwidth"
	var speed int
	if *bw == "" {
		speed = 0
	} else {
		if sp, err := parseBynaryPrefix(*bw); err != nil {
			fmt.Fprintf(os.Stderr, "\n\nParameter Error (bandwidth:%s) %v\n", *bw, err)
			os.Exit(9)
		} else {
			speed = sp
		}
	}

	// Parse "tty"
	tty_regex := regexp.MustCompile(`^.*/`)
	tty_device := "/dev/" + tty_regex.ReplaceAllString(*tty, "")

	// debug
	DEBUG = *debug

	// filenames
	var filename string
	switch len(flag.Args()) {
	case 0:
		filename = ""
	case 1:
		filename = flag.Args()[0]
	default:
		fmt.Fprintf(os.Stderr, "\n\nParameter Error\n") // Todo read more than one file at once
		os.Exit(9)
	}

	if filename == "" {
		*graph = false
	}

	return &option{
		speed:     speed,
		tty:       tty_device,
		silent:    *silent,
		graph:     *graph,
		filename:  filename,
		show_data: *data,
	}
}

func main() {
	option := getOption()

	if option.filename == "" {
		//Check if os.Stdin is piped or from terminal
		fi, err := os.Stdin.Stat() // FileInfo
		if err != nil {
			fmt.Fprintf(os.Stderr, "Stdin FileInfo Error :%v\n", err)
			os.Exit(9)
		}

		if (fi.Mode() & os.ModeCharDevice) != 0 {
			// from terminal
			fmt.Fprintf(os.Stderr, "Can't open file or stdin\n")
			os.Exit(1)
		}
		limitedPipe(os.Stdin, os.Stdout, 0, option)

	} else {
		file, fileinfo, err := openfile(option.filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n\nFile Open Error :%v\n", err)
			os.Exit(9)
		}
		defer file.Close()
		limitedPipe(file, os.Stdout, int(fileinfo.Size()), option)
	}
}

const BUFSIZE = 4096

type readbuf struct {
	length int
	buf    []byte
}

func read(in io.Reader, rb chan readbuf, writing_done <-chan struct{}) {
	reader := bufio.NewReader(in)
	buf := make([]byte, BUFSIZE)
	for {
		if n, err := reader.Read(buf); n == 0 {
			break
		} else if err == io.EOF {
			rb <- readbuf{length: n, buf: buf}
			<-writing_done
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "\n\nFile Read Error :%v\n", err)
			os.Exit(9)
		} else {
			rb <- readbuf{length: n, buf: buf}
			<-writing_done
		}
	}
}

func limitedPipe(in io.Reader, out io.Writer, size int, option *option) {
	ctx := context.Background()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	rbchan := make(chan readbuf)
	reading_done := make(chan struct{})
	writing_done := make(chan struct{})
	go func() {
		read(in, rbchan, writing_done)
		reading_done <- struct{}{}
	}()

	wg := &sync.WaitGroup{}

	//sk := NewSpeedKeeper(time.Now(), speed, size)
	sk := NewSpeedKeeper(ctx, cancel, time.Now(), option.speed, size)
	wg.Add(1)
	go func() {
		sk.run()
		wg.Done()
	}()

	mon := newMonitor(ctx, cancel, sk)
	if option.silent == true {
		mon.setMode("silent")
	}
	if option.graph == true {
		mon.setMode("graph")
	}
	mon.setOption(option)

	wg.Add(1)
	go func() {
		mon.run()
		wg.Done()
	}()

	tick := time.NewTicker(time.Millisecond * time.Duration(50)).C
	readBytes := 0
L:
	for {
		select {
		case rb := <-rbchan:
			if rb.length == 0 {
				continue
			}
			readBytes += rb.length
			out.Write(rb.buf[:rb.length])
			if option.show_data {
				mon.data <- rb.buf[:rb.length]
			}
			sk.curchan <- readBytes
			writing_done <- struct{}{}
			<-sk.killTime()
		case <-tick:
			mon.progress <- struct{}{}
		case <-reading_done:
			mon.progress <- struct{}{}
			cancel()
			break L
		case <-ctx.Done():
			break L
		}
	}
	wg.Wait()
}

//
// The speedKeeper is a goroutine which keeps input/output speed.
//

type speedKeeper struct {
	ctx        context.Context
	cancel     func()
	start      time.Time
	bytePerSec int
	size       int
	current    int
	curchan    chan int
}

func NewSpeedKeeper(ctx context.Context, cancel func(), s time.Time, b int, size int) *speedKeeper {
	return &speedKeeper{
		ctx:        ctx,
		cancel:     cancel,
		start:      s,
		bytePerSec: b,
		size:       size,
		current:    0,
		curchan:    make(chan int),
	}
}

func (sk *speedKeeper) run() {
L:
	for {
		select {
		case curBytes := <-sk.curchan:
			sk.current = curBytes
		case <-sk.ctx.Done():
			break L
		}
	}
}

func (sk *speedKeeper) killTime() <-chan struct{} {
	outchan := make(chan struct{})
	go func() {
		if sk.bytePerSec > 0 {
			//target_duration := time.Duration(float64(curBytes/sk.bytePerSec)) * time.Second //NG
			//target_duration := time.Duration(float64(curBytes*1e9/sk.bytePerSec)) * time.Nanosecond
			//target_duration := time.Duration(float64(curBytes*1000/sk.bytePerSec)) * time.Millisecond
			target_duration := time.Duration(float64(sk.current*1000/sk.bytePerSec)) * time.Millisecond
			current_duration := time.Since(sk.start)
			wait := target_duration - current_duration
			dprintf(" target=%s current=%s\n", target_duration, current_duration)
			if wait > 0 {
				dprintf("Sleep %s\n", wait)
				time.Sleep(wait)
			}
			dprintf(" wait finished %d\n", wait)
		}
		outchan <- struct{}{}
	}()
	return outchan
}

func (sk *speedKeeper) currentSpeed() int {
	//d := int(time.Since(sk.start).Seconds())
	d := int(time.Since(sk.start).Nanoseconds())
	if d == 0 {
		return 0
	}
	return sk.current * 1e9 / d
}

//
// The monitor is a progress monitoring goroutine.
//

type monitor struct {
	ctx      context.Context
	cancel   func()
	tty      io.Writer
	sk       *speedKeeper
	mode     string // Monitor Mode : Standard, Silent, Graphical,
	progress chan struct{}
	data     chan []byte
	option   *option
	width    int
}

func getTty(device string) *os.File {
	//fmt.Fprintf(os.Stderr, "device:%s\n", device)
	tty, err := os.Create(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "File Open Error device:%s error:%v\n", device, err)
	}
	return tty
}

func newMonitor(ctx context.Context, cancel func(), sk *speedKeeper) *monitor {
	return &monitor{
		ctx:      ctx,
		cancel:   cancel,
		sk:       sk,
		progress: make(chan struct{}),
		data:     make(chan []byte),
	}
}

func (mon *monitor) setMode(mode string) {
	mon.mode = mode
}

func (mon *monitor) setOption(option *option) {
	mon.option = option
}

func (mon *monitor) setTty() {
	mon.tty = getTty(mon.option.tty)
}

func (mon *monitor) setWidth(w int) {
	mon.width = w
}

func (mon *monitor) standardProgress() {
	p := ""
	if mon.sk.size > 0 {
		p = fmt.Sprintf("(%3d%%)", int(mon.sk.current*100/mon.sk.size))
	}
	j, bp := inBynaryPrefix(mon.sk.currentSpeed())
	fmt.Fprintf(mon.tty, "\r\033[K[%s]\t%dBytes%s\t@ %.1f%sBps",
		time.Now().Format("2006/01/02 15:04:05.000 MST"),
		mon.sk.current,
		p,
		j,
		bp,
	)
}

func (mon *monitor) getGraphProgress() func() {
	mon.setWidth(int(getWidth() / 2))
	var bar_string string
	for i := 0; i < mon.width; i++ {
		bar_string = bar_string + "*"
	}
	bar := []byte(bar_string)

	return func() {
		p := ""
		if mon.sk.size > 0 {
			p = fmt.Sprintf("(%3d%%)", int(mon.sk.current*100/mon.sk.size))
		}

		j, bp := inBynaryPrefix(mon.sk.currentSpeed())
		fmt.Fprintf(mon.tty, "\r\033[K[%s]\t%dBytes%s\t@ %.1f%sBps\t[%-"+strconv.Itoa(mon.width)+"s]",
			time.Now().Format("2006/01/02 15:04:05.000 MST"),
			mon.sk.current,
			p,
			j,
			bp,
			bar[:int(mon.sk.current*mon.width/mon.sk.size)],
		)
	}
}

func (mon *monitor) showData(buf []byte) {
	if mon.tty != nil {
		fmt.Fprintf(mon.tty, "%s", buf)
	}
}

//implicit type progresser interface {
//	initFunc()
//	pFunc()
//	endFunc()
//}

func (mon *monitor) run() {
	var initFunc, pFunc, endFunc func()
	var showFunc func([]byte)
	switch mon.mode {
	case "silent":
		initFunc = func() {}
		pFunc = func() {}
		showFunc = func(b []byte) {}
		endFunc = func() {}
	case "graph":
		initFunc = func() {
			mon.setTty()
		}
		pFunc = mon.getGraphProgress()
		showFunc = mon.showData
		endFunc = func() {
			fmt.Fprintf(mon.tty, "\n")
		}
	default:
		initFunc = func() {
			mon.setTty()
		}
		pFunc = mon.standardProgress
		showFunc = mon.showData
		endFunc = func() {
			fmt.Fprintf(mon.tty, "\n")
		}
	}
	initFunc()
L:
	for {
		select {
		case <-mon.progress:
			pFunc()
		case <-mon.ctx.Done():
			break L
		case buf := <-mon.data:
			showFunc(buf)
		}
	}
	endFunc()
}
