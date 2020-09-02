你好！
很冒昧用这样的方式来和你沟通，如有打扰请忽略我的提交哈。我是光年实验室（gnlab.com）的HR，在招Golang开发工程师，我们是一个技术型团队，技术氛围非常好。全职和兼职都可以，不过最好是全职，工作地点杭州。
我们公司是做流量增长的，Golang负责开发SAAS平台的应用，我们做的很多应用是全新的，工作非常有挑战也很有意思，是国内很多大厂的顾问。
如果有兴趣的话加我微信：13515810775  ，也可以访问 https://gnlab.com/，联系客服转发给HR。
# IRCdiscord

An IRCd that lets you talk to Discord users.

Essentially an IRC server that connects to Discord to relay messages between an IRC client and Discord.

# Capabilities
Listed below are current features.
- /whois gives information on discord users
- DM support
- Talk in any server/channel
- /list lists all channels in server
- Join all chats in a server by using /join * or /join "*"

# Installation
Build with `go build` and then copy into your $PATH. You can also grab a prebuilt binary above.

# Usage
Run the program, and in your IRC client, connect to `127.0.0.1` with the server password being `<your discord token>:<target discord server id>`.

An example for weechat:
```
/server add discordserver -password=lkajf_343jlksaf43wjalfkjdsaf:348734324
/set irc.server.discordserver.capabilities "server-time"
/set irc.server.discordserver.autoconnect on
/set irc.server.discordserver.autojoin "#channel1,#channel2,#channel3"
/connect discordserver
```
If the server ID is omitted, then it will join a server with no channels but with DM capabilities.

# License
ISC; see LICENSE file.
