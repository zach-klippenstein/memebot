package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	. "github.com/zach-klippenstein/memebot"
)

const (
	SLACK_TOKEN_VAR               = "SLACK_TOKEN"
	IMAGE_DIR_VAR                 = "IMAGES_DIR"
	KEYWORD_PATTERN_VAR           = "KEYWORD_PATTERN"
	IMAGE_SERVER_HOSTNAME_VAR     = "HOSTNAME"
	IMAGE_SERVER_PORT_VAR         = "PORT"
	IMAGE_SERVER_DISPLAY_PORT_VAR = "DISPLAY_PORT"

	DEFAULT_KEYWORD_PATTERN = `(\w+)$`
)

var (
	ImagesDir              string
	KeywordPattern         string
	ImageServerHostname    string
	ImageServerPort        int
	ImageServerDisplayPort int

	ListKeywordsMode = flag.Bool("list-keywords", false, "lists the set of keywords without starting the bot")
	ListMemesMode    = flag.Bool("list-memes", false, "lists all memes' URLs")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage of %s:", os.Args[0])
		fmt.Fprintln(os.Stderr, os.Args[0], "-images path [options...]")
		fmt.Fprintln(os.Stderr, os.Args[0], "-list-keywords")
		fmt.Fprintln(os.Stderr, os.Args[0], "-list-memes")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "The images directory is required, everything else is optional.")
	}

	flag.StringVar(&ImagesDir, "images", "",
		"path of directory containing images named like keyword1[,keyword2,...]. Overrides "+IMAGE_DIR_VAR+" environment variable.")
	flag.StringVar(&KeywordPattern, "keyword-pattern", "",
		"case-insensitive regex with capture groups used to extract keywords from messages. Overrides "+KEYWORD_PATTERN_VAR+" environment variable. Default is "+DEFAULT_KEYWORD_PATTERN)
	flag.StringVar(&ImageServerHostname, "serve-host", "",
		"hostname to use in image links. Overrides "+IMAGE_SERVER_HOSTNAME_VAR+" environment variable.")
	flag.IntVar(&ImageServerPort, "serve-port", 0,
		"port to listen on for serving images. Overrides "+IMAGE_SERVER_PORT_VAR+" environment variable.")
	flag.IntVar(&ImageServerDisplayPort, "serve-display-port", 0,
		"port use in image links. Maybe be different from -serve-port if your load balancer forwards 80 to 5000, e.g. Defaults to the bound port. Overrides "+IMAGE_SERVER_DISPLAY_PORT_VAR+" environment variable.")
}

func main() {
	flag.Parse()

	if ImagesDir == "" {
		ImagesDir = os.Getenv(IMAGE_DIR_VAR)
	}
	if ImagesDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	if KeywordPattern == "" {
		KeywordPattern = os.Getenv(KEYWORD_PATTERN_VAR)
	}
	if KeywordPattern == "" {
		KeywordPattern = DEFAULT_KEYWORD_PATTERN
	}

	if ImageServerHostname == "" {
		ImageServerHostname = os.Getenv(IMAGE_SERVER_HOSTNAME_VAR)
	}

	if ImageServerPort == 0 {
		ImageServerPort, _ = strconv.Atoi(os.Getenv(IMAGE_SERVER_PORT_VAR))
	}
	if ImageServerPort == 0 {
		ImageServerPort = 80
	}

	if ImageServerDisplayPort == 0 {
		ImageServerDisplayPort, _ = strconv.Atoi(os.Getenv(IMAGE_SERVER_DISPLAY_PORT_VAR))
	}
	if ImageServerDisplayPort == 0 {
		ImageServerDisplayPort = ImageServerPort
	}

	port := ":" + strconv.Itoa(ImageServerPort)
	displayPort := ":" + strconv.Itoa(ImageServerDisplayPort)
	router := mux.NewRouter().Host(getHostname() + displayPort).Subrouter()
	rootRoute := router.PathPrefix("/memes/")

	memepository := NewFileServingMemepository(FileServingMemepositoryConfig{
		Path:            ImagesDir,
		ImageExtensions: MakeSet("png", "jpg"),
		Router:          rootRoute.Subrouter(),
	})

	memes, err := memepository.Load()
	if err != nil {
		log.Fatal("error loading memes:", err)
	}

	if *ListKeywordsMode {
		for _, keyword := range memes.Keywords() {
			fmt.Printf("%s (%d)\n", keyword, len(memes.FindByKeyword(keyword)))
		}
		os.Exit(0)
	}

	if *ListMemesMode {
		for _, meme := range memes.Memes() {
			fmt.Printf("%s (%s)\n", meme.URL(), strings.Join(meme.Keywords(), ","))
		}
		os.Exit(0)
	}

	slackToken, found := os.LookupEnv(SLACK_TOKEN_VAR)
	if !found || slackToken == "" {
		log.Fatal("Slack token not found. Set ", SLACK_TOKEN_VAR)
	}

	keywordPattern, err := regexp.Compile("(?i)" + KeywordPattern)
	if err != nil {
		log.Fatalf("error compiling keyword pattern '%s': %s", KeywordPattern, err.Error())
	}

	go func() {
		err := http.ListenAndServe(port, router)
		if err != nil {
			log.Fatal("image server error:", err)
		}
	}()
	rootUrl, _ := rootRoute.URL()
	log.Println("serving images on", rootUrl)

	bot := &MemeBot{
		Parser:   RegexpKeywordParser{keywordPattern},
		Searcher: &MemepositorySearcher{memepository},
		Handler:  DefaultHandler{},
	}

	bot.Dial(slackToken)
	LogConnectionInfo(bot)
	bot.Run(context.Background())
}

func getHostname() string {
	if ImageServerHostname != "" {
		return ImageServerHostname
	}

	host, err := os.Hostname()
	if err != nil {
		log.Fatal("error getting hostname:", err)
	}
	return host
}

func LogConnectionInfo(b *MemeBot) {
	log.Print("memebot ready as @", b.Name(), " (^c to exit)")

	log.Println("member of channels:")
	for _, channel := range b.Channels() {
		if channel.IsMember {
			log.Println("  ", channel.Name)
		}
	}

	log.Println("matching keywords on", b.Parser)
}
