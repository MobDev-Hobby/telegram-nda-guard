package scanner

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

const (
	StateDefault = iota
	StatePhoneRequired
	StatePasswordRequired
	StateCodeRequired
	StateFirstNameRequired
	StateLastNameRequired
)

type BotAuthFlow struct {
	d           *Domain
	state       int
	chatID      int64
	BotChan     chan string
	handlerHook string
}

func (d *Domain) NewBotAuthFlow(
	ctx context.Context,
	chatID int64,
) *BotAuthFlow {

	authFlow := &BotAuthFlow{
		d:       d,
		chatID:  chatID,
		state:   StateDefault,
		BotChan: make(chan string),
	}

	authFlow.handlerHook = d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message == nil {
				return false
			}
			return (*update.Message).ChatID == authFlow.chatID
		},
		authFlow.handler,
	)
	return authFlow
}

func (f *BotAuthFlow) Done(ctx context.Context) {
	f.d.telegramBot.ClearHandler(f.handlerHook)
	close(f.BotChan)
	_ = f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Auth done",
		},
	)
}

func (f *BotAuthFlow) handler(
	_ context.Context,
	update *guard.Update,
) {

	switch f.state {
	case StatePasswordRequired,
		StateCodeRequired,
		StatePhoneRequired,
		StateFirstNameRequired,
		StateLastNameRequired:
		f.BotChan <- update.Message.Text
	}
}

func (f *BotAuthFlow) Phone(
	ctx context.Context,
) (string, error) {

	err := f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Enter the phone number, please:",
		},
	)
	if err != nil {
		return "", fmt.Errorf("can't send message: %w", err)
	}
	f.state = StatePhoneRequired
	select {
	case phone := <-f.BotChan:
		return phone, nil
	case <-ctx.Done():
		return "", context.DeadlineExceeded
	}
}

func (f *BotAuthFlow) Password(
	ctx context.Context,
) (string, error) {

	err := f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Enter password, please:",
		},
	)
	if err != nil {
		return "", fmt.Errorf("can't send message: %w", err)
	}
	f.state = StatePasswordRequired
	select {
	case pass := <-f.BotChan:
		return pass, nil
	case <-ctx.Done():
		return "", context.DeadlineExceeded
	}
}

func (f *BotAuthFlow) SignUp(
	ctx context.Context,
) (auth.UserInfo, error) {

	userInfo := auth.UserInfo{}

	err := f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Enter first name, please:",
		},
	)
	if err != nil {
		return userInfo, fmt.Errorf("can't send message: %w", err)
	}
	f.state = StateFirstNameRequired
	select {
	case userInfo.FirstName = <-f.BotChan:
	case <-ctx.Done():
		return userInfo, context.DeadlineExceeded
	}

	err = f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Enter last name, please:",
		},
	)
	if err != nil {
		return userInfo, err
	}
	f.state = StateLastNameRequired
	select {
	case userInfo.LastName = <-f.BotChan:
	case <-ctx.Done():
		return userInfo, context.DeadlineExceeded
	}

	return userInfo, nil
}

func (f *BotAuthFlow) Code(
	ctx context.Context,
	_ *tg.AuthSentCode,
) (string, error) {

	err := f.d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: f.chatID,
			Text:   "Enter code, please:",
		},
	)
	if err != nil {
		return "", fmt.Errorf("can't send message: %w", err)
	}
	f.state = StateCodeRequired
	select {
	case code := <-f.BotChan:
		return code, nil
	case <-ctx.Done():
		return "", context.DeadlineExceeded
	}
}

func (f *BotAuthFlow) AcceptTermsOfService(
	_ context.Context,
	_ tg.HelpTermsOfService,
) error {

	return nil
}
