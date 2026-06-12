# Hawking Discord Bot

Hawking is a modern, lightweight Discord bot crafted in Go. Originally built to dynamically fetch and display university class schedules, it has now been enhanced with Spotify

## Key Features

- **Dynamic Schedule Retrieval**: Query university class timetables instantly using the public `/jadwal` command.
- **Spotify Cataloging & Archiving**: Extract track metadata (titles, artists) directly from Spotify URLs using the Spotify Web API and persist them into your personal database.
- **Granular Authorization Engine**: A secure permission layer restricting administrative and music management commands to the bot owner and whitelisted members.

---

## Command Registry

| Command          | Scope                   | Description                                                        |
| :--------------- | :---------------------- | :----------------------------------------------------------------- |
| `/jadwal`        | **Public**              | Retrieves and renders the university class schedule.               |
| `/save [url]`    | **Owner & Whitelisted** | Parses a Spotify track URL, fetches its metadata, and archives it. |
| `/mysongs`       | **Owner & Whitelisted** | Lists, plays, cycles, and deletes saved tracks.                    |
| `/grant [user]`  | **Owner Only**          | Grants an individual permissions to use the music commands.        |
| `/revoke [user]` | **Owner Only**          | Rescinds a user's permission to interact with the music commands.  |

---

`/mysongs` supports `mode:list page:<number>`, `mode:play`, `mode:stopcycle`, and `mode:delete id:<saved_track_id>`. List mode is paginated and includes navigation buttons.
