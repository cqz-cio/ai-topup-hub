package application

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/captcha/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

func TestRegistrationImageChallengeIsRequiredAndSingleUse(t *testing.T) {
	setting := settingssecurity.NormalizeCaptchaSetting(settingssecurity.CaptchaSetting{
		Provider: constants.CaptchaProviderImage,
		Scenes:   settingssecurity.CaptchaSceneSetting{RegisterSendCode: true},
	})
	service := NewService(settingReaderStub{setting: setting}, config.CaptchaConfig{}, nil)
	scene := constants.CaptchaSceneRegisterSendCode
	if err := service.Verify(scene, contract.VerifyPayload{}, ""); err != contract.ErrRequired {
		t.Fatalf("missing challenge: got %v, want ErrRequired", err)
	}
	for _, wrongFirst := range []bool{false, true} {
		challenge, err := service.GenerateImageChallenge()
		if err != nil {
			t.Fatal(err)
		}
		if challenge.CaptchaID == "" || !strings.HasPrefix(challenge.ImageBase64, "data:image/") {
			t.Fatal("challenge must contain an ID and an image")
		}
		answer := service.imageStore.Get(challenge.CaptchaID, false)
		if answer == "" {
			t.Fatal("generated challenge has no stored answer")
		}
		payload := contract.VerifyPayload{CaptchaID: challenge.CaptchaID, CaptchaCode: answer}
		if wrongFirst {
			incorrect := payload
			incorrect.CaptchaCode = "incorrect-answer"
			if err := service.Verify(scene, incorrect, ""); err != contract.ErrInvalid {
				t.Fatalf("incorrect challenge: got %v, want ErrInvalid", err)
			}
		} else if err := service.Verify(scene, payload, ""); err != nil {
			t.Fatalf("valid registration challenge rejected: %v", err)
		}
		if err := service.Verify(scene, payload, ""); err != contract.ErrInvalid {
			t.Fatalf("consumed challenge can be reused: got %v, want ErrInvalid", err)
		}
	}
	if err := service.Verify(constants.CaptchaSceneLogin, contract.VerifyPayload{}, ""); err != nil {
		t.Fatalf("registration setting unexpectedly enabled login captcha: %v", err)
	}
}
