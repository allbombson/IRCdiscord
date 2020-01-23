package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
)

// is there a better way?
func addHandlers(s *discordgo.Session) {
	s.AddHandler(guildMembersChunk)
	s.AddHandler(messageCreate)
	s.AddHandler(messageDelete)
	s.AddHandler(messageUpdate)
	s.AddHandler(channelCreate)
	s.AddHandler(channelDelete)
	s.AddHandler(channelUpdate)
	s.AddHandler(guildRoleCreate)
	s.AddHandler(guildRoleDelete)
	s.AddHandler(guildRoleUpdate)
	s.AddHandler(guildMemberAdd)
	s.AddHandler(guildMemberRemove)
	s.AddHandler(guildMemberUpdate)
}

func guildMembersChunk(session *discordgo.Session, chunk *discordgo.GuildMembersChunk) {
	fmt.Printf("processing guildMembersChunk: %d members\n", len(chunk.Members))
	var guildSession *guildSession
	var err error
	guildSession, err = getGuildSession(session.Token, chunk.GuildID)
	if err != nil {
		fmt.Println(err)
		return
	}
	cachedGuild, _ := session.State.Guild(chunk.GuildID)
	cachedGuild.Members = append(cachedGuild.Members, chunk.Members...)
	removeDuplicateMembers(&cachedGuild.Members)
	for _, member := range chunk.Members {
		guildSession.addMember(member)
	}
	fmt.Printf("we have %d members of %d\n", len(cachedGuild.Members), guildSession.guild.MemberCount)
	if len(cachedGuild.Members) >= guildSession.guild.MemberCount {
		guildSession.membersDone = true
	}
}

func removeDuplicateMembers(list *[]*discordgo.Member) {
	found := make(map[string]bool)
	j := 0
	for i, x := range *list {
		if !found[x.User.ID] {
			found[x.User.ID] = true
			(*list)[j] = (*list)[i]
			j++
		}
	}
	*list = (*list)[:j]
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	var guildSession *guildSession
	var err error
	if message.GuildID != "" {
		guildSession, err = getGuildSession(session.Token, message.GuildID)
	} else if message.ChannelID != "" {
		guildSession, err = getGuildSession(session.Token, "")
	}
	if err != nil {
		return
	}
	guildSession.addMessage(message.Message)
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		date, err := message.Message.Timestamp.Parse()
		if err != nil {
			return
		}
		sendMessageFromDiscordToIRC(date, conn, message.Message, "", "")
	}
}

func messageUpdate(session *discordgo.Session, message *discordgo.MessageUpdate) {
	guildSession, err := getGuildSession(session.Token, message.GuildID)
	if err != nil {
		return
	}
	oldMessage, err := guildSession.getMessage(message.ChannelID, message.ID)
	if err != nil {
		return
	}
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		date, err := message.Timestamp.Parse()
		if err != nil {
			return
		}
		if !oldMessage.Pinned && message.Message.Pinned {
			sendMessageFromDiscordToIRC(date, conn, oldMessage, "\x0312message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+" was pinned\x0f:\n", "")
		} else if oldMessage.Pinned && !message.Message.Pinned {
			sendMessageFromDiscordToIRC(date, conn, oldMessage, "\x0312message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+" was unpinned\x0f:\n", "")
		} else {
			sendMessageFromDiscordToIRC(date, conn, oldMessage, "\x0312message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f:\n", "")
			sendMessageFromDiscordToIRC(date, conn, message.Message, "\x0312was edited to:\n", "")
		}
	}
	guildSession.addMessage(message.Message)
}

func messageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	guildSession, err := getGuildSession(session.Token, message.GuildID)
	if err != nil {
		return
	}
	for _, conn := range guildSession.conns {

		if conn == nil {
			continue
		}
		oldMessage, err := guildSession.getMessage(message.ChannelID, message.ID)
		if err != nil {
			return
		}
		sendMessageFromDiscordToIRC(time.Now(), conn, oldMessage, "\x0304message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f \x0304in this channel was deleted:\n", "")
		return
	}
}

func channelCreate(session *discordgo.Session, channel *discordgo.ChannelCreate) {
	guildSession, err := getGuildSession(session.Token, channel.GuildID)
	if err != nil {
		return
	}
	guildSession.addChannel(channel.Channel)
}

func channelDelete(session *discordgo.Session, channel *discordgo.ChannelDelete) {
	// TODO: kick user from channel
	guildSession, err := getGuildSession(session.Token, channel.GuildID)
	if err != nil {
		return
	}
	guildSession.removeChannel(channel.Channel)
}

func channelUpdate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	// TODO: handle channel name changes somehow
	guildSession, err := getGuildSession(session.Token, channel.GuildID)
	if err != nil {
		return
	}
	guildSession.updateChannel(channel.Channel)
}

func guildRoleCreate(session *discordgo.Session, role *discordgo.GuildRoleCreate) {
	guildSession, err := getGuildSession(session.Token, role.GuildID)
	if err != nil {
		return
	}
	guildSession.addRole(role.Role)
}

func guildRoleDelete(session *discordgo.Session, role *discordgo.GuildRoleDelete) {
	guildSession, err := getGuildSession(session.Token, role.GuildID)
	if err != nil {
		return
	}
	guildSession.removeRole(role.RoleID)
}

func guildRoleUpdate(session *discordgo.Session, role *discordgo.GuildRoleUpdate) {
	// TODO: handle channel name changes somehow
	guildSession, err := getGuildSession(session.Token, role.GuildID)
	if err != nil {
		return
	}
	guildSession.updateRole(role.Role)
}

func guildMemberAdd(session *discordgo.Session, member *discordgo.GuildMemberAdd) {
	fmt.Println("processing GuildMemberAdd")
	fmt.Printf("username: %s#%s | nick: %s\n", member.User.Username, member.User.Discriminator, getIRCNick(member.Nick))
	guildSession, err := getGuildSession(session.Token, member.GuildID)
	if err != nil {
		return
	}
	guildSession.addMember(member.Member)
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		for ircChannel := range conn.channels {
			channelName := guildSession.channelMap.GetName(ircChannel)
			fmt.Println(channelName)
			conn.sendJOIN(
				conn.getNick(member.User),
				convertDiscordUsernameToIRCRealname(member.Member.User.Username),
				member.Member.User.ID,
				channelName,
			)
		}
		return
	}
}

func guildMemberUpdate(session *discordgo.Session, member *discordgo.GuildMemberUpdate) {
	fmt.Println("processing GuildMemberUpdate")
	guildSession, err := getGuildSession(session.Token, member.GuildID)
	if err != nil {
		return
	}

	oldNick := guildSession.getNick(member.User)
	guildSession.updateMember(member.Member)
	newNick := guildSession.getNick(member.User)
	if oldNick != newNick {
		for _, conn := range guildSession.conns {
			if conn == nil {
				continue
			}
			conn.sendNICK(
				oldNick,
				oldNick,
				member.User.ID,
				newNick,
			)
			return
		}
	}
	// TODO: handle role changes?
}

func guildMemberRemove(session *discordgo.Session, member *discordgo.GuildMemberRemove) {
	fmt.Println("processing GuildMemberRemove")
	guildSession, err := getGuildSession(session.Token, member.GuildID)
	if err != nil {
		return
	}
	guildSession.removeMember(member.Member)
	fmt.Printf("username: %s#%s | nick: %s\n", member.User.Username, member.User.Discriminator, getIRCNick(member.Nick))
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		conn.sendQUIT(
			conn.getNick(member.User),
			convertDiscordUsernameToIRCRealname(member.Member.User.Username),
			member.Member.User.ID,
			"Quit",
		)
		return
	}
	// TODO: send part like guildMemberAdd
}
