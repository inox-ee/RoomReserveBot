package api

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"

	"ventus-inc/Ventus_Office_ReserveBot/util"

	"github.com/slack-go/slack"
)

func slackVerificationMiddleware(config util.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, config.SlackSigningSecret)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bodyReader := io.TeeReader(r.Body, &verifier)
		body, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err = verifier.Ensure(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(w, r)
	}
}
