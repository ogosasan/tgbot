package main

import (
	"context"
	"fmt"
	tgbotapi "github.com/skinass/telegram-bot-api/v5"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var (
	BotToken   = "6672756651:AAGAHfWTnEFdbC49V8DdhwxmF8fbS-ZTvTw"
	WebhookURL = "https://7369-95-24-150-68.ngrok-free.app"
)

var commands = map[string]func(update tgbotapi.Update, storage Storage) map[int64]string{
	"/tasks":    ViewAllTasks,
	"/new":      CreateTask,
	"/my":       ViewMyTasks,
	"/owner":    ViewTasksCreatedByMe,
	"/assign":   AssignTask,
	"/unassign": UnassignTask,
	"/resolve":  ResolveTask,
}

var (
	executableTasks []infAboutCommand
	increment       = 1
)

type infAboutCommand struct {
	NameOfCommand string
	Assign        bool
	Creator       User
	Executor      User
	ID            int
}

type Storage struct {
	mapForStorage map[int64]string
	mutex         *sync.Mutex
}

type User struct {
	ID   int64
	Name string
}

func AddInformationAboutCommand(update tgbotapi.Update) {
	var executableTask infAboutCommand
	executableTask.ID = increment
	executableTask.NameOfCommand = strings.SplitN(update.Message.Text, " ", 2)[1]
	executableTask.Creator.Name = update.Message.Chat.UserName
	executableTask.Creator.ID = update.Message.Chat.ID
	executableTask.Executor.Name = update.Message.Chat.UserName
	executableTask.Executor.ID = update.Message.Chat.ID
	executableTask.Assign = false
	executableTasks = append(executableTasks, executableTask)
	increment += 1
}

func (p *infAboutCommand) UpdateInformationAboutCommand(update tgbotapi.Update) {
	switch GetCommand(update) {
	case "/assign":
		p.Executor.Name = update.Message.Chat.UserName
		p.Executor.ID = update.Message.Chat.ID
		p.Assign = true
	case "/unassign":
		p.Executor.Name = p.Creator.Name
		p.Executor.ID = p.Creator.ID
		p.Assign = false
	}
}

func GetCommand(update tgbotapi.Update) string {
	command := update.Message.Text
	if strings.Contains(command, " ") {
		command = strings.SplitN(command, " ", 2)[0]
	}
	if strings.Contains(command, "_") {
		command = strings.SplitN(command, "_", 2)[0]
	}
	return command
}

func UpdateHandler(update tgbotapi.Update, storage Storage) map[int64]string {
	command := GetCommand(update)
	storage.ClearData()
	answer := commands[command](update, storage)
	return answer
}

func CreateTask(update tgbotapi.Update, storage Storage) map[int64]string {
	command := strings.SplitN(update.Message.Text, " ", 2)[1]
	resCommand := fmt.Sprintf(`Задача "%s" создана, id=%d`, command, increment)
	storage.mutex.Lock()
	storage.mapForStorage[update.Message.Chat.ID] = resCommand
	storage.mutex.Unlock()
	AddInformationAboutCommand(update)
	return storage.mapForStorage
}

func ViewAllTasks(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	if len(executableTasks) == 0 {
		storage.mapForStorage = map[int64]string{
			update.Message.Chat.ID: "Нет задач",
		}
		return storage.mapForStorage
	}
	resCommand := ""
	for ind, infAbCommand := range executableTasks {
		if ind > 0 {
			resCommand += "\n\n"
		}
		resCommand += fmt.Sprintf("%d. %s by @%s", infAbCommand.ID, infAbCommand.NameOfCommand, infAbCommand.Creator.Name)
		if infAbCommand.Assign {
			if infAbCommand.Executor.ID == update.Message.Chat.ID {
				resCommand += "\nassignee: я"
				resCommand += fmt.Sprintf("\n/unassign_%d /resolve_%d", infAbCommand.ID, infAbCommand.ID)
			} else {
				resCommand += fmt.Sprintf("\nassignee: @%s", infAbCommand.Executor.Name)
			}
		} else {
			resCommand += fmt.Sprintf("\n/assign_%d", infAbCommand.ID)
		}
		storage.mapForStorage[update.Message.Chat.ID] = resCommand
	}
	return storage.mapForStorage
}

func AssignTask(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	for ind, infAbCommand := range executableTasks {
		indOfCommand, err := strconv.Atoi(strings.SplitN(update.Message.Text, "_", 2)[1])
		if err != nil {
			panic(err)
		}
		if indOfCommand == infAbCommand.ID {
			storage.mapForStorage[update.Message.Chat.ID] = fmt.Sprintf(`Задача "%s" назначена на вас`, infAbCommand.NameOfCommand)
			if !infAbCommand.Assign && infAbCommand.Creator.ID == update.Message.Chat.ID {
				executableTasks[ind].UpdateInformationAboutCommand(update)
				return storage.mapForStorage
			}
			storage.mapForStorage[infAbCommand.Executor.ID] = fmt.Sprintf(`Задача "%s" назначена на @%s`, infAbCommand.NameOfCommand, update.Message.Chat.UserName)
			executableTasks[ind].UpdateInformationAboutCommand(update)
			return storage.mapForStorage
		}
	}
	return nil
}

func UnassignTask(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	for ind, infAbCommand := range executableTasks {
		indOfCommand, err := strconv.Atoi(strings.SplitN(update.Message.Text, "_", 2)[1])
		if err != nil {
			panic(err)
		}
		if infAbCommand.ID == indOfCommand && infAbCommand.Executor.ID == update.Message.Chat.ID {
			storage.mapForStorage[update.Message.Chat.ID] = "Принято"
			storage.mapForStorage[infAbCommand.Creator.ID] = fmt.Sprintf(`Задача "%s" осталась без исполнителя`, infAbCommand.NameOfCommand)
			executableTasks[ind].UpdateInformationAboutCommand(update)
			return storage.mapForStorage
		}
	}
	storage.mapForStorage[update.Message.Chat.ID] = "Задача не на вас"
	return storage.mapForStorage
}

func ResolveTask(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	for index, infAbCommand := range executableTasks {
		indOfCommand, err := strconv.Atoi(strings.SplitN(update.Message.Text, "_", 2)[1])
		if err != nil {
			panic(err)
		}
		if infAbCommand.ID == indOfCommand && infAbCommand.Executor.ID == update.Message.Chat.ID {
			storage.mapForStorage[update.Message.Chat.ID] = fmt.Sprintf(`Задача "%s" выполнена`, infAbCommand.NameOfCommand)
			storage.mapForStorage[infAbCommand.Creator.ID] = fmt.Sprintf(`Задача "%s" выполнена @%s`, infAbCommand.NameOfCommand, infAbCommand.Executor.Name)
			executableTasks = slices.Delete(executableTasks, index, index+1)
			return storage.mapForStorage
		}
	}
	return storage.mapForStorage
}

func ViewMyTasks(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	var resCommand string
	for _, infAbCommand := range executableTasks {
		if infAbCommand.Executor.ID == update.Message.Chat.ID {
			resCommand = fmt.Sprintf("%d. %s by @%s", infAbCommand.ID, infAbCommand.NameOfCommand, infAbCommand.Creator.Name)
			resCommand += fmt.Sprintf("\n/unassign_%d /resolve_%d", infAbCommand.ID, infAbCommand.ID)
			storage.mapForStorage[update.Message.Chat.ID] = resCommand
		}
	}
	return storage.mapForStorage
}

func ViewTasksCreatedByMe(update tgbotapi.Update, storage Storage) map[int64]string {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	var resCommand string
	for _, infAbCommand := range executableTasks {
		if infAbCommand.Creator.ID == update.Message.Chat.ID {
			resCommand = fmt.Sprintf("%d. %s by @%s", infAbCommand.ID, infAbCommand.NameOfCommand, infAbCommand.Creator.Name)
			resCommand += fmt.Sprintf("\n/assign_%d", infAbCommand.ID)
			storage.mapForStorage[update.Message.Chat.ID] = resCommand
		}
	}
	return storage.mapForStorage
}

func (s *Storage) ClearData() {
	for k := range s.mapForStorage {
		delete(s.mapForStorage, k)
	}
}

func startTaskBot(ctx context.Context) error {
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		log.Fatalf("NewBotAPI failed: %s", err)
		return err
	}

	wh, err := tgbotapi.NewWebhook(WebhookURL)
	if err != nil {
		log.Fatalf("NewWebhook failed: %s", err)
		return err
	}

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatalf("SetWebhook failed: %s", err)
		return err
	}

	updates := bot.ListenForWebhook("/")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	go func() {
		err = http.ListenAndServe(":"+port, nil)
		if err != nil {
			panic(err)
		}
	}()
	if BotToken == "_golangcourse_test" {
		go func() {
			err = http.ListenAndServe(":8081", nil)
			if err != nil {
				panic(err)
			}
		}()
	}
	StorageMap := Storage{
		mapForStorage: map[int64]string{},
		mutex:         &sync.Mutex{},
	}
	for {
		select {
		case update := <-updates:
			go func(update tgbotapi.Update, storage Storage) {
				response := UpdateHandler(update, storage)
				for id, message := range response {
					messageForTg := tgbotapi.NewMessage(id, message)
					_, sendErr := bot.Send(messageForTg)
					if sendErr != nil {
						panic(sendErr)
					}
				}
			}(update, StorageMap)
		case <-ctx.Done():
			return nil
		}
	}
}

func main() {
	err := startTaskBot(context.Background())
	if err != nil {
		panic(err)
	}
}
