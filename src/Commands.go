package main

import (
	"fmt"
	"strings"
	"regexp"
)

const RPL_WELCOME = "001"
const RPL_ENDOFNAMES = "366"
const RPL_NAMREPLY = "353"
const RPL_TOPIC = "332"
const RPL_LISTSTART = "321"
const RPL_LIST = "322"
const RPL_LISTEND = "323"

const ERR_NICKNAMEINUSE = "433"
const ERR_NONICKNAMEGIVEN = "431"
const ERR_NOSUCHNICK = "401"
const ERR_BADCHANNELKEY = "475"
const ERR_NOSUCHCHANNEL = "403"
const ERR_NOTONCHANNEL = "442"
const ERR_USERONCHANNEL = "443"

func (c *connection) handle_cmd_pass(password string) {
	// https://tools.ietf.org/html/rfc1459#section-4.1.1
	c.session.password = password
}

func (c *connection) handle_cmd_nick(nickname string) (resp_code string, resp_str string) {
	// https://tools.ietf.org/html/rfc1459#section-4.1.2
	for _, e := range current_users {
		if (e.nickname == nickname) {
			return ERR_NICKNAMEINUSE, ":Nickname is already in use."
		}
	}
	fmt.Println("User nickname: ", c.session.nickname, " updated as ", nickname)
	c.session.nickname = nickname
	// c.session.client = c
	current_users = append(current_users, c.session)
	return
}

func (c *connection) handle_cmd_user(username string, hostname string, servername string, realname string) (resp_code string, resp_str string){
	// https://tools.ietf.org/html/rfc1459#section-4.1.3
	if (c.session.nickname == "") {
		return ERR_NONICKNAMEGIVEN, fmt.Sprintf(":No nickname given")
	}
	c.session.username = username
	c.session.realname = realname
	// return RPL_WELCOME, ":Welcome to the Internet Relay Network"
	return RPL_WELCOME, fmt.Sprintf(":Welcome to the Internet Relay Network %s!%s@%s", c.session.nickname, c.session.username, c.server.prefix)
}

func get_channel(channel_name string) (channel_ptr *channel){
	for _, c := range current_channels {
		if (c.name == channel_name) {
			return c
		}
	}
	return nil
}

func get_channel_nicknames(channel_ *channel) (users_nicknames []string) {
	var r []string

	for _, u := range channel_.subscribed_users {
		r = append(r, u.nickname)
	}
	return r
}

func (c *connection) handle_cmd_names(channels string) (nicknames_fmt []string) {
	// https://tools.ietf.org/html/rfc1459#section-4.2.5
	var resp []string

	channels_list := strings.Split(channels, ",")
	if (channels != "") {
		for _, c := range channels_list {
			if (c[0] == '#') {
				chan_ptr := get_channel(c[1:])
				channel_nicknames := get_channel_nicknames(chan_ptr)
				channel_nicknames_fmt := strings.Join(channel_nicknames, " ")
				resp = append(resp, fmt.Sprintf("= %s :%s", c, channel_nicknames_fmt))
			}
		}
	} else {
		var nicknames []string
		for _, e := range current_connections {
			if (e.session != nil) {
				nicknames = append(nicknames, e.session.nickname)
			}
		}
		resp = append(resp, fmt.Sprintf("%s %s :%s", "*", "*", strings.Join(nicknames, " ")))
	}
	return resp
}

func parse_last_argument(raw_line string) (match string) {
	r, _ := regexp.Compile(`(PRIVMSG [#$]?[\w,]*)[ ]:([\w\s]{1,})$`)
	rslt := r.FindStringIndex(raw_line)
	if (len(rslt) > 0) {
		fmt.Println("Found", rslt, " - ", rslt[1], " - ", raw_line[rslt[1]:])
		return "TEST"
		// return raw_line[rslt[1]:]
	}
	return ""
}

func (c *connection) handle_cmd_privmsg(receiver string, raw_line string) {
	// https://tools.ietf.org/html/rfc1459#section-4.4.1
	receivers := strings.Split(receiver, ",")
	text := parse_last_argument(raw_line)
	fmt.Println("Send message", text, "- To receivers ", receivers)
	for _, e := range receivers {
		if (e[0] != '#' && e[0] != '$') {
			for _, u := range current_users {
				if (u.nickname == e) {
					fmt.Println("Send to user : ", u, "client", u.client)
					u.client.send(fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s", c.session.nickname, c.session.username, c.server.prefix, receiver, text))
					return
				}
			}
			c.send(c.format_resp(ERR_NOSUCHNICK, ":No such nick/channel"))
		} else {
			// Handle host / server mask
			print("TODO")
		}
	}
}

func (c *connection) handle_cmd_join(channelname string) (resp_code string, resp_str string){
	// https://tools.ietf.org/html/rfc1459#section-1.3
	// https://tools.ietf.org/html/rfc1459#section-4.2.1
	if (channelname[0] != '#') {
		c.send(c.format_resp(ERR_BADCHANNELKEY, ":Channel name must start with '#' (server channel) or '&' (distributed channel)"))
		return 
	}
	newchan := get_channel(channelname)
	if (newchan == nil) {
		newchan = &channel{name: channelname, topic: "", subscribed_users: []*user{c.session}}
		current_channels = append(current_channels, newchan)
	} else {
		for _, u := range newchan.subscribed_users {
			if (u == c.session) {
				c.send(c.format_resp(ERR_USERONCHANNEL, c.session.nickname, newchan.name, ":is already on channel"))
				return
			}
		}
		newchan.subscribed_users = append(newchan.subscribed_users, c.session)
	}
	c.send(c.format_resp(RPL_TOPIC, c.session.nickname, newchan.name, ":", newchan.topic))
	c.send(c.format_resp(RPL_NAMREPLY, fmt.Sprintf("%s = %s :%s", c.session.nickname, newchan.name, get_channel_nicknames(newchan))))
	c.send(c.format_resp(RPL_ENDOFNAMES, c.session.nickname, newchan.name, ":End of NAMES list"))
	channel_broadcast(newchan, c.format_resp(fmt.Sprintf(":%s!~%s@%s", c.session.nickname, c.session.username, c.server.prefix), "JOIN", channelname))
	// c.send(c.format_resp("NOTICE", c.session.nickname, fmt.Sprintf(":[%s] Welcome to the %s channel", channelname, channelname)))
	c.handle_cmd_notice(fmt.Sprintf(":[%s] Welcome to the %s channel", channelname, channelname))
	return
}

func (c *connection) handle_cmd_list() (resp_code string, resp_str string){
	// https://tools.ietf.org/html/rfc1459#section-4.2.6
	c.send(c.format_resp(RPL_LISTSTART, c.session.nickname, "Channel :Users Topic"))
	for _, e := range current_channels {
		c.send(c.format_resp(RPL_LIST, c.session.nickname, e.name, fmt.Sprintf("%d :%s", len(e.subscribed_users), e.topic)))
	}
	c.send(c.format_resp(RPL_LISTEND, c.session.nickname, ":End of LIST"))
	return
}


func (c *connection) handle_cmd_part(channelname string) (resp_code string, resp_str string){
	chan_ := get_channel(channelname)
	if (chan_ == nil) {
		c.send(c.format_resp(ERR_NOSUCHCHANNEL, c.session.nickname, channelname, ":No such channel"))
	} else {
		for i, e := range chan_.subscribed_users {
			if (e == c.session) {
				fmt.Printf("User %s left channel %s\n", c.session.nickname, chan_.name)
				chan_.subscribed_users[i] = chan_.subscribed_users[len(chan_.subscribed_users) - 1]
				chan_.subscribed_users[len(chan_.subscribed_users) - 1] = nil
				chan_.subscribed_users = chan_.subscribed_users[:len(chan_.subscribed_users) - 1]

				channel_broadcast(chan_, c.format_resp(fmt.Sprintf(":%s!~%s@%s", c.session.nickname, c.session.username, c.server.prefix), "PART", channelname))

				if (len(chan_.subscribed_users) <= 0 && chan_.name != "#home") {
					for j, f := range current_channels {
						if (f == chan_) {
							current_channels[j] = current_channels[len(current_channels) - 1]
							current_channels[len(current_channels) - 1] = nil
							current_channels = current_channels[:len(current_channels) - 1]
						}
					}
				}
				return
			}
		}
		c.send(c.format_resp(ERR_NOTONCHANNEL, c.session.nickname, channelname, ":You're not on that channel"))
	}
	return
}

//func (c *connection) handle_cmd_topic(channelname string, topic string) {
func (c *connection) handle_cmd_topic(channelname string, topic string) {
	chan_ := get_channel(channelname)
	if (chan_ == nil) {
		c.send(c.format_resp(ERR_NOSUCHCHANNEL, c.session.nickname, channelname, ":No such channel"))
	} else {
		for _, u := range chan_.subscribed_users {
			if (u == c.session) {
				if (topic == "") {
					c.send(c.format_resp(RPL_TOPIC, c.session.nickname, chan_.name, ":", chan_.topic))
				} else {
					chan_.topic = topic
					c.send(c.format_resp(fmt.Sprintf(":%s!~%s@%s", c.session.nickname, c.session.username, c.server.prefix), "TOPIC", channelname, topic))
				}
				return
			}
		}
		c.send(c.format_resp(ERR_NOTONCHANNEL, c.session.nickname, channelname, ":You're not on that channel"))
	}
}

func (c *connection) handle_cmd_notice(text string) {
	c.send(c.format_resp("NOTICE", c.session.nickname, text))
}
