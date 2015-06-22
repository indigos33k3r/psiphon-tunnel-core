package upstreamproxy

import (
	"encoding/base64"
	"errors"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/upstreamproxy/go-ntlm/ntlm"
	"net/http"
	"strings"
)

type NTLMHttpAuthState int

const (
	NTLM_HTTP_AUTH_STATE_CHALLENGE_RECEIVED NTLMHttpAuthState = iota
	NTLM_HTTP_AUTH_STATE_RESPONSE_TYPE1_GENERATED
	NTLM_HTTP_AUTH_STATE_RESPONSE_TYPE3_GENERATED
)

type NTLMHttpAuthenticator struct {
	state NTLMHttpAuthState
}

func newNTLMAuthenticator() *NTLMHttpAuthenticator {
	return &NTLMHttpAuthenticator{state: NTLM_HTTP_AUTH_STATE_CHALLENGE_RECEIVED}
}

func (a *NTLMHttpAuthenticator) authenticate(req *http.Request, resp *http.Response, username, password string) error {
	challenges, err := parseAuthChallenge(resp)

	challenge, ok := challenges["NTLM"]
	if !ok {
		return errors.New("upstreamproxy: Bad proxy response, no NTLM challenge for NTLMHttpAuthenticator")
	}

	var ntlmMsg []byte

	session, err := ntlm.CreateClientSession(ntlm.Version2, ntlm.ConnectionOrientedMode)
	if err != nil {
		return err
	}
	if a.state == NTLM_HTTP_AUTH_STATE_CHALLENGE_RECEIVED {
		//generate TYPE 1 message
		negotiate, err := session.GenerateNegotiateMessage()
		if err != nil {
			return err
		}
		ntlmMsg = negotiate.Bytes()
		a.state = NTLM_HTTP_AUTH_STATE_RESPONSE_TYPE1_GENERATED
		req.Header.Set("Proxy-Authorization", "NTLM "+base64.StdEncoding.EncodeToString(ntlmMsg))
		return nil
	} else if a.state == NTLM_HTTP_AUTH_STATE_RESPONSE_TYPE1_GENERATED {
		// Parse username for domain in form DOMAIN\username
		var NTDomain, NTUser string
		parts := strings.SplitN(username, "\\", 2)
		if len(parts) == 2 {
			NTDomain = parts[0]
			NTUser = parts[1]
		} else {
			NTDomain = ""
			NTUser = username
		}
		challengeBytes, err := base64.StdEncoding.DecodeString(challenge)
		if err != nil {
			return err
		}
		session.SetUserInfo(NTUser, password, NTDomain)
		ntlmChallenge, err := ntlm.ParseChallengeMessage(challengeBytes)
		if err != nil {
			return err
		}
		session.ProcessChallengeMessage(ntlmChallenge)
		authenticate, err := session.GenerateAuthenticateMessage()
		if err != nil {
			return err
		}
		ntlmMsg = authenticate.Bytes()
		a.state = NTLM_HTTP_AUTH_STATE_RESPONSE_TYPE3_GENERATED
		req.Header.Set("Proxy-Authorization", "NTLM "+base64.StdEncoding.EncodeToString(ntlmMsg))
		return nil
	}

	return errors.New("upstreamproxy: Authorization is not accepted by the proxy server")
}
