package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/tadeokondrak/irc"
)

func (c *ircConn) handleWHOIS(m *irc.Message) {
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.WHOIS, "Not enough parameters")
		return
	}
	userID := c.guildSession.userMap.GetSnowflake(m.Params[0])
	if userID == "" {
		c.sendNOTICE("Failed to find that user")
		return
	}
	user, err := c.getUser(userID)
	_ = user
	if err != nil {
		c.sendNOTICE("Failed to find that user")
		return
	}
	c.sendRPL(irc.RPL_WHOISUSER, c.getNick(user), c.getRealname(user), user.ID, "*", user.String())
}

func (c *ircConn) handleCAP(m *irc.Message) {
	const ERR_INVALIDCAPCMD = "410"
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.JOIN, "Not enough parameters")
		return
	}
	if len(m.Params) > 1 && m.Params[1] == "302" {
		c.user.supportsCap302 = true
	}
	switch m.Params[0] {
	case irc.CAP_LS:
		c.user.capBlocked = true
		c.sendCAP(irc.CAP_LS, strings.Join(supportedCapabilities, " "))
	case irc.CAP_LIST:
		supportedCaps := []string{}
		for key := range c.user.supportedCapabilities {
			supportedCaps = append(supportedCaps, key)
		}
		c.sendCAP(irc.CAP_LIST, strings.Join(supportedCaps, " "))
	case irc.CAP_REQ:
		if len(m.Params) < 2 {
			c.sendERR(ERR_INVALIDCAPCMD, irc.CAP_REQ, "Not enough parameters")
			return
		}
		caps := strings.Split(m.Params[1], " ")
		success := true
		approvedCaps := []string{}
		for _, cap := range caps {
			approvedCaps = append(approvedCaps, cap)
			if cap == "" || strings.HasPrefix(cap, "-") {
				continue
			}
			for _, supportedCap := range supportedCapabilities {
				if cap == supportedCap {
					goto end // this would be a continue if this weren't a nested loop
				}
			}
			success = false
		end:
		}
		if success {
			for _, cap := range approvedCaps {
				if strings.HasPrefix(cap, "-") {
					c.user.supportedCapabilities[cap[1:]] = false
				} else {
					c.user.supportedCapabilities[cap] = true
				}
			}
			c.sendCAP(irc.CAP_ACK, strings.Join(approvedCaps, " "))
		} else {
			c.sendCAP(irc.CAP_NAK, strings.Join(approvedCaps, " "))
		}
	case irc.CAP_END:
		c.user.capBlocked = false
		if c.readyToRegister() {
			c.register()
		}
	}
	return
}

func (c *ircConn) handleMOTD() {
	c.sendERR(irc.ERR_NOMOTD, "MOTD file is missing")
}

func (c *ircConn) handleNICK(m *irc.Message) {
	if c.loggedin {
		// TODO
		return
	}
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NONICKNAMEGIVEN, "No nickname given")
		return
	}

	if false {
		c.sendERR(irc.ERR_ERRONEUSNICKNAME, "Erroneus nickname")
		return
	}

	c.user.nick = m.Params[0]

	if c.readyToRegister() {
		c.register()
		return
	}
}

func (c *ircConn) handleUSER(m *irc.Message) {
	if c.loggedin {
		c.sendERR(irc.ERR_ALREADYREGISTRED, irc.PASS, "You may not reregister")
		return
	}

	if len(m.Params) < 4 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.USER, "Not enough parameters")
		return
	}

	c.user.username = m.Params[0]
	c.user.realname = m.Params[3]

	if c.readyToRegister() {
		c.register()
		return
	}
}

func (c *ircConn) handlePASS(m *irc.Message) {
	if c.loggedin {
		c.sendERR(irc.ERR_ALREADYREGISTRED, irc.PASS, "You may not reregister")
		return
	}

	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.PASS, "Not enough parameters")
		return
	}

	c.user.password = m.Params[0]

	if c.readyToRegister() {
		c.register()
	}
}

func (c *ircConn) handleTOPIC(m *irc.Message) {
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.JOIN, "Not enough parameters")
		return
	}

	if len(m.Params) > 1 {
		// TODO
		return
	}

	channelName := m.Params[0]

	channelID := c.guildSession.channelMap.GetSnowflake(channelName)

	c.channelsMutex.Lock()
	if !c.channels[channelID] {
		c.channelsMutex.Unlock()
		c.sendERR(irc.ERR_NOTONCHANNEL, "You're not on that channel")
		return
	}
	c.channelsMutex.Unlock()

	channel, err := c.getChannel(channelID)
	if err != nil {
		return
	}

	topic := convertDiscordTopicToIRC(channel.Topic, c)

	if topic != "" {
		c.sendRPL(irc.RPL_TOPIC, channelName, topic)
		c.sendRPL(irc.RPL_TOPICWHOTIME, channelName, "noone", "0")
	}
}

func (c *ircConn) handleJOIN(m *irc.Message) {
	// TODO: rewrite
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.JOIN, "Not enough parameters")
		return
	}

	channelID := c.guildSession.channelMap.GetSnowflake(m.Params[0])

	c.channelsMutex.Lock()
	if c.channels[channelID] {
		// user already on channel
		c.channelsMutex.Unlock()
		return
	}
	c.channelsMutex.Unlock()
	//Lord Forgive me for what i'm about to do
	//TODO: Fix this asap its utter shit also note to self learn fucking go to avoid this.
	if m.Params[0] == "*" {
        for channelName := range c.guildSession.channelMap.GetSnowflakeMap() {
            discordChannelID := c.guildSession.channelMap.GetSnowflake(channelName)
            if discordChannelID == "" {
                c.sendERR(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
                continue
            }

            discordChannel, err := c.getChannel(discordChannelID)
            if err != nil {
                c.sendNOTICE(fmt.Sprint(err))
                fmt.Println("error fetching channel data")
                continue
            }

            c.channelsMutex.Lock()
            c.channels[discordChannelID] = true
            c.channelsMutex.Unlock()

            c.sendJOIN("", "", "", channelName)

            go c.handleTOPIC(&irc.Message{
                Command: irc.TOPIC,
                Params:  []string{channelName},
            })

            go func(c *ircConn, channel *discordgo.Channel) {
                messages, err := c.session.ChannelMessages(channel.ID, 100, "", "", "")
                if err != nil {
                    c.sendNOTICE("There was an error getting messages from Discord.")
                    return
                }

                channelName := c.guildSession.channelMap.GetName(channel.ID)
                if channelName == "" {
                    c.sendNOTICE("This shouldn't happen (1). If you see this, report it as a bug.")
                    return
                }

                tag := uuid.New().String()
                if c.user.supportedCapabilities["batch"] {
                    c.sendBATCH(true, tag, "chathistory", channelName)
                }
                for i := len(messages); i != 0; i-- { // Discord sends them in reverse order
                    date, err := messages[i-1].Timestamp.Parse()
                    if err != nil {
                        continue
                    }
                    sendMessageFromDiscordToIRC(date, c, messages[i-1], "", tag)
                }
                if c.user.supportedCapabilities["batch"] {
                    c.sendBATCH(false, tag)
                }
            }(c, discordChannel)
            go c.handleNAMES(&irc.Message{Command: irc.NAMES, Params: []string{channelName}})
        }
	} else {
        for _, channelName := range strings.Split(m.Params[0], ",") {
            discordChannelID := c.guildSession.channelMap.GetSnowflake(channelName)
            if discordChannelID == "" {
                c.sendERR(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
                continue
            }

            discordChannel, err := c.getChannel(discordChannelID)
            if err != nil {
                c.sendNOTICE(fmt.Sprint(err))
                fmt.Println("error fetching channel data")
                continue
            }

            c.channelsMutex.Lock()
            c.channels[discordChannelID] = true
            c.channelsMutex.Unlock()

            c.sendJOIN("", "", "", channelName)

            go c.handleTOPIC(&irc.Message{
                Command: irc.TOPIC,
                Params:  []string{channelName},
            })

            go func(c *ircConn, channel *discordgo.Channel) {
                messages, err := c.session.ChannelMessages(channel.ID, 100, "", "", "")
                if err != nil {
                    c.sendNOTICE("There was an error getting messages from Discord.")
                    return
                }

                channelName := c.guildSession.channelMap.GetName(channel.ID)
                if channelName == "" {
                    c.sendNOTICE("This shouldn't happen (1). If you see this, report it as a bug.")
                    return
                }

                tag := uuid.New().String()
                if c.user.supportedCapabilities["batch"] {
                    c.sendBATCH(true, tag, "chathistory", channelName)
                }
                for i := len(messages); i != 0; i-- { // Discord sends them in reverse order
                    date, err := messages[i-1].Timestamp.Parse()
                    if err != nil {
                        continue
                    }
                    sendMessageFromDiscordToIRC(date, c, messages[i-1], "", tag)
                }
                if c.user.supportedCapabilities["batch"] {
                    c.sendBATCH(false, tag)
                }
            }(c, discordChannel)
            go c.handleNAMES(&irc.Message{Command: irc.NAMES, Params: []string{channelName}})
        }
	}
}

func (c *ircConn) handlePART(m *irc.Message) {
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NEEDMOREPARAMS, irc.PART, "Not enough parameters")
		return
	}

	var reason string
	if len(m.Params) > 1 {
		reason = m.Params[1]
	}

	for _, channelName := range strings.Split(m.Params[0], ",") {
		discordChannelID := c.guildSession.channelMap.GetSnowflake(channelName)
		if discordChannelID == "" {
			c.sendERR(irc.ERR_NOSUCHCHANNEL, "No such channel")
			continue
		}
		if _, exists := c.channels[discordChannelID]; !exists {
			c.sendERR(irc.ERR_NOTONCHANNEL, "You're not on that channel")
			continue
		}
		c.channelsMutex.Lock()
		c.channels[discordChannelID] = false
		c.channelsMutex.Unlock()
		c.sendPART("", "", "", channelName, reason)
	}
}

func (c *ircConn) handlePING(m *irc.Message) {
	if len(m.Params) > 0 {
		c.sendPONG(m.Params[0])
	} else {
		c.sendPONG("")
	}
}

func (c *ircConn) handlePONG(m *irc.Message) {
	if len(m.Params) < 1 {
		return
	}
	c.lastPONG = m.Params[0]
}

func (c *ircConn) handleNAMES(m *irc.Message) {
	if len(m.Params) < 1 {
		c.sendRPL(irc.RPL_ENDOFNAMES, "*", "End of /NAMES list")
		return
	}
	var ircNickArray []string
	if c.guildSessionType == guildSessionGuild {
		for c.guildSession.membersDone == false {
			time.Sleep(5 * time.Second)
		}
		ircNicks := c.guildSession.userMap.GetSnowflakeMap()
		ircNickArray = []string{}
		for nick := range ircNicks {
			ircNickArray = append(ircNickArray, nick)
		}
	} else if c.guildSessionType == guildSessionDM {
		ircNickArray = []string{}
		channelID := c.guildSession.channelMap.GetSnowflake(m.Params[0])
		channel, err := c.getChannel(channelID)
		if err != nil {
			// TODO: error
			return
		}
		for _, user := range channel.Recipients {
			ircNickArray = append(ircNickArray, c.getNick(user))
		}
		ircNickArray = append(ircNickArray, c.getNick(c.selfUser))
	}
	done := false
	for i := 0; !done; {
		_ircNicks := []string{}
		for len(strings.Join(_ircNicks, " ")) < 400 {
			if i >= len(ircNickArray) {
				done = true
				break
			}
			nick := ircNickArray[i]
			_ircNicks = append(_ircNicks, nick)
			i++
		}
		c.sendRPL(irc.RPL_NAMREPLY, "=", m.Params[0], strings.Join(_ircNicks, " "))
	}

	c.sendRPL(irc.RPL_ENDOFNAMES, m.Params[0], "End of /NAMES list")
}

func (c *ircConn) handleLIST(m *irc.Message) {
	if len(m.Params) > 0 && m.Params[0] != "" {
		// TODO
		return
	}
	c.sendRPL(irc.RPL_LISTSTART, "Channels", "Users  Name")
	for ircChannel, discordChannelID := range c.guildSession.channelMap.GetSnowflakeMap() {
		discordChannel, err := c.getChannel(discordChannelID)
		if err != nil {
			c.sendNOTICE(fmt.Sprint(err))
			fmt.Println("error getting channel")
			continue
		}

		c.sendRPL(
			irc.RPL_LIST,
			ircChannel,
			strconv.Itoa(c.guildSession.userMap.Length()),
			convertDiscordTopicToIRC(discordChannel.Topic, c),
		)
	}
	c.sendRPL(irc.RPL_LISTEND, "End of /LIST")
}

func (c *ircConn) handlePRIVMSG(m *irc.Message) {
	if len(m.Params) < 1 {
		c.sendERR(irc.ERR_NORECIPIENT, "No recipient given (PRIVMSG)")
		return
	}
	if len(m.Params) < 2 || m.Params[1] == "" {
		c.sendERR(irc.ERR_NOTEXTTOSEND, "No text to send")
		return
	}

	channel := c.guildSession.channelMap.GetSnowflake(m.Params[0])
	if channel == "" {
		c.sendERR(irc.ERR_NOSUCHCHANNEL, m.Params[0], "No such channel")
		return
	}

	content := convertIRCMessageToDiscord(c, m.Params[1])

	addRecentlySentMessage(c, channel, content)

	_, err := c.session.ChannelMessageSend(channel, content)
	if err != nil {
		// TODO: map common discord errors to irc errors
		c.sendNOTICE("There was an error sending your message.")
		fmt.Println(err)
		return
	}
}
