package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/exp/slices"

	"github.com/bwmarrin/discordgo"
)

var Token string
var s *discordgo.Session

type EditingState struct {
	user        string
	itemMessage *discordgo.Message
	noteMessage *discordgo.Message
}

var currentlyEditing *EditingState

const PREFIX = "!"
const EMOJI_CHECK = '🛒'
const EMOJI_DELETE = '🗑'
const EMOJI_EDIT = '✏'
const EMOJI_CHECKED = '✅'
const EMOJI_UNCHECKED = '◻'

func init() {
	dat, err := os.ReadFile("token.secret")
	if err != nil {
		fmt.Println("error reading token file,", err)
		os.Exit(1)
	}
	Token = strings.TrimSpace(string(dat))
}

func main() {
	var err error
	s, err = discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	s.AddHandler(onMessageCreate)
	s.AddHandler(onMessageReactionAdd)
	s.AddHandler(onMessageReactionRemove)

	s.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions

	err = s.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	defer s.Close()

	fmt.Printf("%v\nLogged in as %v#%v\n", time.Now().String(), s.State.User.Username, s.State.User.Discriminator)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if m.Author.ID == s.State.User.ID || strings.HasPrefix(m.Content, "#") {
		return
	}

	if strings.HasPrefix(m.Content, PREFIX) {
		splits := strings.Fields(m.Content[len(PREFIX):])
		switch splits[0] {
		case "trackchannel":
			fallthrough
		case "addchannel":
			if addChannel(m.ChannelID) {
				s.ChannelMessageSend(m.ChannelID, "channel added")
			} else {
				s.ChannelMessageSend(m.ChannelID, "channel already added")
			}
			break
		case "untrackchannel":
			fallthrough
		case "removechannel":
			if removeChannel(m.ChannelID) {
				s.ChannelMessageSend(m.ChannelID, "channel removed")
			} else {
				s.ChannelMessageSend(m.ChannelID, "channel not tracked")
			}
		default:
			fmt.Printf("Unknown command %v\n", splits[0])
		}
		return
	}

	if !isChannelActivated(m.ChannelID) {
		return
	}

	if currentlyEditing != nil && currentlyEditing.noteMessage.ChannelID == m.ChannelID {
		previousContent := trimItem(currentlyEditing.itemMessage.Content)
		s.ChannelMessageEdit(currentlyEditing.itemMessage.ChannelID, currentlyEditing.itemMessage.ID,
			string(EMOJI_UNCHECKED)+" "+m.Content)
		fmt.Printf("Edited %v to %v  (in %v by %v)\n", previousContent, m.Content, m.ChannelID, m.Author.Username)
		s.ChannelMessageDelete(m.ChannelID, m.ID)
		cancelEditing()

		return
	}

	items := strings.Split(m.Content, "\n")

	for _, v := range items {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}
		if strings.HasPrefix(v, "#") {
			s.ChannelMessageSend(m.ChannelID, "**__"+strings.Trim(v, "*_")+"__**")
		} else {
			fmt.Printf("Added %v  (in %v by %v)\n", v, m.ChannelID, m.Author.Username)
			msg, _ := s.ChannelMessageSend(m.ChannelID, string(EMOJI_UNCHECKED)+" "+v)
			addButtons(msg)
		}
	}
	s.ChannelMessageDelete(m.ChannelID, m.ID)
}

func trimItem(item string) string {
	return strings.TrimRight(strings.TrimLeft(item, "~ "+string(EMOJI_CHECKED)+string(EMOJI_UNCHECKED)), "~ ")
}

func checkItem(msg *discordgo.Message) {
	item := trimItem(msg.Content)
	s.ChannelMessageEdit(msg.ChannelID, msg.ID, string(EMOJI_CHECKED)+" ~~"+item+"~~")
}

func uncheckItem(msg *discordgo.Message) {
	item := trimItem(msg.Content)
	s.ChannelMessageEdit(msg.ChannelID, msg.ID, string(EMOJI_UNCHECKED)+" "+item)
}

func cancelEditing() {
	if currentlyEditing == nil {
		return
	}
	s.ChannelMessageDelete(currentlyEditing.noteMessage.ChannelID, currentlyEditing.noteMessage.ID)
	if currentlyEditing.itemMessage.Reactions[slices.IndexFunc(currentlyEditing.itemMessage.Reactions,
		func(rs *discordgo.MessageReactions) bool { return rs.Emoji.Name == string(EMOJI_EDIT) })].Count > 1 {
		s.MessageReactionsRemoveEmoji(currentlyEditing.itemMessage.ChannelID, currentlyEditing.itemMessage.ID, string(EMOJI_EDIT))
		s.MessageReactionAdd(currentlyEditing.itemMessage.ChannelID, currentlyEditing.itemMessage.ID, string(EMOJI_EDIT))
	}
	currentlyEditing = nil

}

func onMessageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}

	msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		fmt.Println("error fetching message,", err)
		return
	}

	item := trimItem(msg.Content)
	isEditingMsg := currentlyEditing != nil && currentlyEditing.itemMessage.ID == msg.ID
	switch r.Emoji.Name {
	case string(EMOJI_CHECK):
		if isEditingMsg {
			cancelEditing()
		}
		fmt.Printf("Checked %v  (in %v by %v)\n", item, r.ChannelID, r.Member.User.Username)
		checkItem(msg)
	case string(EMOJI_DELETE):
		if isEditingMsg {
			s.ChannelMessageDelete(currentlyEditing.noteMessage.ChannelID, currentlyEditing.noteMessage.ID)
			currentlyEditing = nil
		}
		fmt.Printf("Removed %v  (in %v by %v)\n", item, r.ChannelID, r.Member.User.Username)
		s.ChannelMessageDelete(r.ChannelID, r.MessageID)
	case string(EMOJI_EDIT):
		if isEditingMsg {
			return
		}
		fmt.Printf("Editing %v (in %v by %v)\n", item, r.ChannelID, r.Member.User.Username)
		nmsg, err := s.ChannelMessageSend(msg.ChannelID, "Editing: `"+item+"`")
		if err != nil {
			fmt.Println("error sending message,", err)
			return
		}
		currentlyEditing = &EditingState{user: r.UserID, itemMessage: msg, noteMessage: nmsg}
	}
}

func onMessageReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}

	msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		fmt.Println("error fetching message,", err)
		return
	}
	switch r.Emoji.Name {
	case string(EMOJI_CHECK):
		index := slices.IndexFunc(msg.Reactions, func(rs *discordgo.MessageReactions) bool { return rs.Emoji.Name == string(EMOJI_CHECK) })
		if index == -1 || msg.Reactions[index].Count < 2 {
			user, err := s.User(r.UserID)
			var username string
			if err != nil {
				fmt.Println("error fetching user,", err)
				username = r.UserID
			} else {
				username = user.Username
			}
			fmt.Printf("Unchecked %v  (in %v by %v)\n", trimItem(msg.Content), r.ChannelID, username)
			uncheckItem(msg)
		}
	case string(EMOJI_EDIT):
		if currentlyEditing != nil && currentlyEditing.itemMessage.ID == r.MessageID {
			cancelEditing()
		}
	}
}

func addButtons(msg *discordgo.Message) {
	s.MessageReactionAdd(msg.ChannelID, msg.ID, string(EMOJI_CHECK))
	s.MessageReactionAdd(msg.ChannelID, msg.ID, string(EMOJI_DELETE))
	s.MessageReactionAdd(msg.ChannelID, msg.ID, string(EMOJI_EDIT))
}
