package postmark

import (
    "blacklist"
    "bytes"
    "cleanup"
    "encoding/json"
    "env"
    "fmt"
    "io/ioutil"
    "job"
    "log"
    "net/http"
    "os"
    "safely"
    "util"
)

type Any interface{}

const (
    MaxAttachmentSize = 10485760
    Subject           = "convert"
    Endpoint          = "https://api.postmarkapp.com/email"
    AuthHeader        = "X-Postmark-Server-Token"
    FriendlyMessage   = "Sorry, email sending failed."
)

var from = env.Get("FROM")
var token = env.Get("POSTMARK_TOKEN")
var logger = log.New(os.Stdout, "[postmark] ", log.LstdFlags|log.Lmicroseconds)
var client http.Client

func readFile(path string) []byte {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        logger.Panicf("Failed reading file: %s", err.Error())
    }
    return data
}

func setupHeaders(req *http.Request) {
    req.Header.Add("Accept", "application/json")
    req.Header.Add("Content-Type", "application/json")
    req.Header.Add(AuthHeader, token)
}

func Mail(j *job.Job) {
    go safely.Do(logger, j, FriendlyMessage, func() {
        if stat, err := os.Stat(j.MobiFilePath()); err != nil {
            logger.Panicf("Something weird happen. Mobi is missing in postmark.go: %s", err.Error())
        } else {
            if stat.Size() > MaxAttachmentSize {
                blacklist.Blacklist(j.Url.String())
                failFriendly("Sorry, this article is too big to send!")
            }
        }

        payload := map[string]Any{
            "From":     from,
            "To":       j.Email,
            "Subject":  Subject,
            "TextBody": fmt.Sprintf("Straight to your Kindle! %s: %s", j.Title, j.Url),
            "Attachments": []Any{
                map[string]Any{
                    "Name":        j.MobiFilename(),
                    "ContentType": "application/octet-stream",
                    "Content":     readFile(j.MobiFilePath()),
                },
            },
        }

        var buffer bytes.Buffer
        json.NewEncoder(&buffer).Encode(payload)

        req, err := http.NewRequest("POST", Endpoint, &buffer)
        if err != nil {
            logger.Panicf("Making HTTP Request failed: %s", err.Error())
        }

        setupHeaders(req)
        resp, err := client.Do(req)
        if err != nil {
            logger.Panicf("Postmark failed: %s", err.Error())
        }

        defer resp.Body.Close()
        answer := util.ParseJSON(resp.Body, func(err error) {
            logger.Panicf("Something bad happened with Postmark: %s", err.Error())
        })

        if answer["ErrorCode"] != nil {
            code := int(answer["ErrorCode"].(float64))
            switch code {
            case 0:
                // All is well
            case 300:
                blacklist.Blacklist(j.Email)
                failFriendly("Your email appears invalid. Please try carefully remaking the bookmarklet.")
            default:
                logger.Panicf("Unknown error code from Postmark: %d, %s", code, answer)
            }
        }

        j.Progress("All done! Grab your Kindle and hang tight!")
        cleanup.Clean(j)
    })
}
