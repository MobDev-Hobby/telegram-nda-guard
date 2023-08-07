package processor_access_control_demo

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
	phone       string
	bot         *bot.Bot
	state       int
	chatId      int64
	BotChan     chan string
	handlerHook string
}

func (d *Domain) NewBotAuthFlow(
	chatId int64,
) *BotAuthFlow {

	authFlow := &BotAuthFlow{
		bot:     d.telegramBot.GetBot(),
		chatId:  chatId,
		state:   StateDefault,
		BotChan: make(chan string),
	}

	authFlow.handlerHook = d.telegramBot.GetBot().RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			return (*update.Message).Chat.ID == authFlow.chatId
		},
		authFlow.handler,
	)
	return authFlow
}

func (f *BotAuthFlow) Done() {
	f.bot.UnregisterHandler(f.handlerHook)
	close(f.BotChan)
}

func (f *BotAuthFlow) handler(
	_ context.Context,
	_ *bot.Bot,
	update *models.Update,
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
	_, err := f.bot.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID: f.chatId,
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
	_, err := f.bot.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID: f.chatId,
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

	_, err := f.bot.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID: f.chatId,
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

	_, err = f.bot.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID: f.chatId,
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
	_, err := f.bot.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID: f.chatId,
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
