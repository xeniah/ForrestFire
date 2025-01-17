package kindlegen

import (
    "fmt"
    T "html/template"
    "job"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "postmark"
    "runtime"
    "safely"
    "util"
)

const (
    FriendlyMessage = "Sorry, conversion failed."
    Tmpl            = `
<html>
    <head>
        <meta content="text/html, charset=utf-8" http-equiv="Content-Type" />
        <meta content="{{.Author}} ({{.Domain}})" name="author" />
        <title>{{.Title}}</title>
    </head>
    <body>
        <h1>{{.Title | html}}</h1>
        {{.HTML}}
        <hr />
        <p>Originally from <a href="{{.Url}}">{{.Url}}</a></p>
        <p>Sent with <a href="http://Tinderizer.com/">Tinderizer</a></p>
        <p>Generated at {{.Now}}</p>
    </body>
</html>
`
)

var kindlegen string
var logger = log.New(os.Stdout, "[kindlegen] ", log.LstdFlags|log.Lmicroseconds)
var template *T.Template

func init() {
    var err error
    kindlegen, err = filepath.Abs(fmt.Sprintf("vendor/kindlegen-%s", runtime.GOOS))
    if err != nil {
        panic(err)
    }
    template = T.Must(T.New("kindle").Parse(Tmpl))
}

func openFile(path string) *os.File {
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        logger.Panicf("Failed opening file: %s", err.Error())
    }
    return file
}

func writeHTML(j *job.Job) {
    file := openFile(j.HTMLFilePath())
    defer file.Close()
    if err := template.Execute(file, j); err != nil {
        logger.Panicf("Failed rendering HTML to file: %s", err.Error())
    }
}

func Convert(j *job.Job) {
    go safely.Do(logger, j, FriendlyMessage, func() {
        writeHTML(j)
        cmd := exec.Command(kindlegen, []string{j.HTMLFilename()}...)
        cmd.Dir = j.Root()
        out, err := cmd.CombinedOutput()
        if !util.FileExists(j.MobiFilePath()) {
            logger.Panicf("Failed running kindlegen: %s {output=%s}", err.Error(), string(out))
        }
        j.Progress("Conversion complete...")
        postmark.Mail(j)
    })
}
