package moderation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type ModlogAction struct {
	Prefix string
	Emoji  string
	Color  int

	Footer string
}

func (m ModlogAction) String() string {
	str := m.Emoji + m.Prefix
	if m.Footer != "" {
		str += " (" + m.Footer + ")"
	}

	return str
}

var (
	MAMute           = ModlogAction{Prefix: "Muted", Emoji: "🔇", Color: 0x57728e}
	MAUnmute         = ModlogAction{Prefix: "Unmuted", Emoji: "🔊", Color: 0x62c65f}
	MAKick           = ModlogAction{Prefix: "Kicked", Emoji: "👢", Color: 0xf2a013}
	MABanned         = ModlogAction{Prefix: "Banned", Emoji: "🔨", Color: 0xd64848}
	MAUnbanned       = ModlogAction{Prefix: "Unbanned", Emoji: "🔓", Color: 0x62c65f}
	MAWarned         = ModlogAction{Prefix: "Warned", Emoji: "⚠", Color: 0xfca253}
	MATimeoutAdded   = ModlogAction{Prefix: "Timed out", Emoji: "⏱", Color: 0x9b59b6}
	MATimeoutRemoved = ModlogAction{Prefix: "Timeout removed from", Emoji: "⏱", Color: 0x9b59b6}
	MAGiveRole       = ModlogAction{Prefix: "", Emoji: "➕", Color: 0x53fcf9}
	MARemoveRole     = ModlogAction{Prefix: "", Emoji: "➖", Color: 0x53fcf9}
)

func CreateModlogEmbed(config *Config, author *discordgo.User, action ModlogAction, target *discordgo.User, reason, logLink string) error {
	channelID := config.IntActionChannel()
	guildID := config.GetGuildID() // SHANE: capture this variable
	if channelID == 0 {
		return nil
	}

	emptyAuthor := false
	if author == nil {
		emptyAuthor = true
		author = &discordgo.User{
			ID:            0,
			Username:      "Unknown",
			Discriminator: "????",
		}
	}

	if reason == "" {
		reason = "(no reason specified)"
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s#%s (ID %d)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: discordgo.EndpointUserAvatar(target.ID, target.Avatar),
		},
		Color: action.Color,
		Description: fmt.Sprintf("**%s%s** %s#%s *(ID %d)*\n📄**Reason:** %s",
			action.Emoji, action.Prefix, target.Username, target.Discriminator, target.ID, reason),
	}

	if logLink != "" {
		embed.Description += " ([Logs](" + logLink + "))"
	}

	if action.Footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: action.Footer,
		}
	}

	// SHANE: log all timeouts as warnings
	// if this is a Timed out modlog, then record a warning
	if action.Prefix == "Timed out" {
		authorUsername := "Unknown"

		if author != nil {
			authorUsername = author.Username + "#" + author.Discriminator
		}

		timeoutReason := "**USER TIMED OUT**"

		if reason != "" {
			timeoutReason = timeoutReason + ": " + reason
		}

		warning := &WarningModel{
			GuildID:               guildID,
			UserID:                discordgo.StrID(target.ID),
			AuthorID:              discordgo.StrID(author.ID),
			AuthorUsernameDiscrim: authorUsername,

			Message: timeoutReason,
		}

		// Create the entry in the database
		err := common.GORM.Create(warning).Error
		if err != nil {
			return common.ErrWithCaller(err)
		}
	}
	// SHANE: end of edits

	m, err := common.BotSession.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeUnknownChannel) {
			// disable the modlog
			config.ActionChannel = ""
			config.Save(guildID) // SHANE: use the captured variable
			return nil
		}
		return err
	}

	if emptyAuthor {
		placeholder := fmt.Sprintf("Assign an author and reason to this using **`reason %d your-reason-here`**", m.ID)
		updateEmbedReason(nil, placeholder, embed)
		_, err = common.BotSession.ChannelMessageEditEmbed(channelID, m.ID, embed)
	}
	return err
}

var (
	logsRegex = regexp.MustCompile(`\(\[Logs\]\(.*\)\)`)
)

func updateEmbedReason(author *discordgo.User, reason string, embed *discordgo.MessageEmbed) {
	const checkStr = "📄**Reason:**"

	index := strings.Index(embed.Description, checkStr)
	withoutReason := embed.Description[:index+len(checkStr)]

	logsLink := logsRegex.FindString(embed.Description)
	if logsLink != "" {
		logsLink = " " + logsLink
	}

	embed.Description = withoutReason + " " + reason + logsLink

	if author != nil {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s#%s (ID %d)", author.Username, author.Discriminator, author.ID),
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		}
	}
}
