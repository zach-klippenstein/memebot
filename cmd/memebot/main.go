package main

import (
	"flag"
	"fmt"
	"log"
	"net"
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
	DefaultPort = 8080

	SlackTokenVar = "SLACK_TOKEN"

	DefaultKeywordPattern = `(\w+)$`
)

var ImageExtensions = []string{"jpg", "png", "gif"}

var (
	ImagesDir = flag.String("images", "",
		"Path of directory containing images named like keyword1[,keyword2,...].")

	KeywordPattern = flag.String("keyword-pattern", DefaultKeywordPattern,
		"Case-insensitive regex with capture groups used to extract keywords from messages.")

	ImageServerHostname = flag.String("serve-host", "",
		"Hostname to use in image links.")

	ImageServerPort = flag.Int("serve-port", DefaultPort,
		"Port to listen on for serving images.")

	ImageServerDisplayPort = flag.Int("serve-display-port", 0,
		"Port use in image links. Maybe be different from -serve-port if your load balancer forwards 80 to 5000, e.g. Defaults to serve-port.")

	OnlyReplyToMentions = flag.Bool("require-mention", true,
		"If true, messages that don't mention bot will be ignored. If you set this, make sure to specify keyword-pattern!")

	ListKeywordsMode = flag.Bool("list-keywords", false,
		"Lists the set of keywords without starting the bot")

	ListMemesMode = flag.Bool("list-memes", false,
		"Lists all memes' URLs")

	ServeOnlyMode = flag.Bool("serve-only", false,
		"Runs the image server without the bot for debugging.")
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
}

func main() {
	flag.Parse()

	if *ImagesDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *ImageServerHostname == "" {
		host, err := os.Hostname()
		if err != nil {
			log.Fatal("error getting hostname:", err)
		}
		*ImageServerHostname = host
	}

	if *ImageServerDisplayPort == 0 {
		*ImageServerDisplayPort = *ImageServerPort
	}

	router := initRouter(*ImageServerHostname, *ImageServerDisplayPort)
	rootRoute := router.PathPrefix("/memes/")
	memepository := NewFileServingMemepository(FileServingMemepositoryConfig{
		Path:            *ImagesDir,
		ImageExtensions: MakeSet(ImageExtensions...),
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

	port := ":" + strconv.Itoa(*ImageServerPort)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal("image server error:", err)
	}
	rootUrl, _ := rootRoute.URL()
	log.Println("image server listening on port", port)
	log.Println("serving images on", rootUrl)

	defer func() {
		log.Println("exiting...")
	}()

	if *ServeOnlyMode {
		err = http.Serve(listener, router)
		if err != nil {
			log.Fatal("image server error:", err)
		}
	} else {
		go func() {
			err := http.Serve(listener, router)
			if err != nil {
				log.Fatal("image server error:", err)
			}
		}()

		startBot(memepository)
	}
}

func initRouter(hostname string, displayPort int) *mux.Router {
	routerAddr := fmt.Sprintf("%s:%d", hostname, displayPort)
	router := mux.NewRouter().Host(routerAddr).Subrouter()

	// Basic "are you alive?" check.
	router.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("health check from %s (X-Forwarded-For: %s)", req.RemoteAddr, req.Header["X-Forwarded-For"])
		fmt.Fprintln(w, "Everything looks good!")
		return
	})

	return router
}

func startBot(memepository Memepository) {
	slackToken := os.Getenv(SlackTokenVar)
	if slackToken == "" {
		log.Fatal("Slack token not found. Set ", SlackTokenVar)
	}

	// Make the regexp case-insensitive.
	keywordPattern, err := regexp.Compile("(?i)" + *KeywordPattern)
	if err != nil {
		log.Fatalf("error compiling keyword pattern '%s': %s", *KeywordPattern, err.Error())
	}

	if !*OnlyReplyToMentions {
		log.Println("WARNING: filtering by mentions is disabled. may be spammy.")
	}

	bot := &MemeBot{
		Parser:           RegexpKeywordParser{keywordPattern},
		Searcher:         &MemepositorySearcher{memepository},
		ErrorHandler:     DefaultErrorHandler{},
		ParseAllMessages: !*OnlyReplyToMentions,
	}

	bot.Dial(slackToken)
	logBotInfo(bot)
	bot.Run(context.Background())
}

func logBotInfo(b *MemeBot) {
	log.Print("memebot ready as @", b.Name(), " (^c to exit)")

	log.Println("member of channels:")
	for _, channel := range b.Channels() {
		if channel.IsMember {
			log.Println("  ", channel.Name)
		}
	}

	log.Println("matching keywords on", b.Parser)
}
