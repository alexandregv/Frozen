package main

import (
	"fmt"
	"strings"
)

const RPL_WELCOME = "001"
const RPL_ENDOFNAMES = "366"
const RPL_NAMREPLY = "353"

const ERR_NICKNAMEINUSE = "433"
const ERR_NONICKNAMEGIVEN = "431"
const ERR_NOSUCHNICK = "401"


func (c *connection) handle_cmd_pass(password string) {
	// https://tools.ietf.org/html/rfc1459#section-4.1.1
	c.session.password = password
}

func (c *connection) handle_cmd_nick(nickname string) (resp_code string, resp_str string) {
	// https://tools.ietf.org/html/rfc1459#section-4.1.2
	for _, e := range current_users {
		if (e.nickname == nickname) {
			return ERR_NICKNAMEINUSE, "Nickname is already in use."
		}
	}
	fmt.Println("User nickname: ", c.session.nickname, " updated as ", nickname)
	c.session.nickname = nickname
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

func (c *connection) handle_cmd_privmsg(receiver string, text string) {
	// https://tools.ietf.org/html/rfc1459#section-4.4.1
	receivers := strings.Split(receiver, ",")
	for _, e := range receivers {
		if (e[0] != '#' && e[0] != '$') {
			for _, u := range current_users {
				if (u.nickname == e) {
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