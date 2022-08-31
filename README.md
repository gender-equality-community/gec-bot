# Gender Equality Community Whatsapp Bot

The GEC Bot does two things:

1. It receives WhatsApp messages from people who want to talk to the GEC anonymously
2. It sends responses back to people anonymously too

![Infrastructure diagram showing how GEC components talk to one another](doc/gec-bots-arch.svg)


## Anonymisation

For each new recipient we generate a random code name using the Diceware Password Generator, as per:

```golang
l, err = diceware.Generate(3)
if err != nil {
    return
}

id = strings.Join(l, "-")
```

We then check whether this ID is already present in our database. This gives keys like:

```txt
overhand-subdivide-thaw
promotion-basically-unreal
clumsily-tag-gizmo
return-lyricist-sixtieth
helmet-gothic-linguist
frugality-pediatric-overstate
subzero-plastic-sadness
sliding-dairy-sleet
endurance-ferry-election
unlatch-childhood-gristle
```

These are used to group messages from a recipient later on, through slack.

**However**

The process of generating an ID and assigning it to a WhatsApp recipient is not a one-way transformation. With access to either the burner phone driving this app, or the underlying database, its possible to figure out who sent what message. This is unavoidable, and good security practice is necessary.

## On Redis Streams

This application passes messages along via redis streams; these are lightweight, as quick as we need them, and can be run in cluster. This is important; by segregating as much as possible from the outside world/ outside users we can keep user data secure.


## Deployment

Deployments are manual for now.
