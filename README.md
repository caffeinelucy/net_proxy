# net_proxy

## why
a friend was in need of a proxy, for pop3 and smtp. so this is a network proxy for smtp, generic tcp but no http. input is plain, output tls. enjoy.

## why no http?
because http proxies exist on mass. in fact if you look for "smtp proxy" dozens of http proxies show up and no single smtp one.

## what does it do?
for every "route" defined in the config file, it creates a little server that listens for incomming connections and creates a sort of forwarding worker connection. the incomming traffic is supposed to be unencrypted while the outgoing traffic will be tls encrypted.
tcp is a generic type which works for most kinds of protocols while smtp is a bit more advanced specifically for smtp connections.

## why no incomming tls?
there's really only 3 reasons for you to run this tool:

- you have some ancient software incapable of handling proper, modern secure tls. just let this tool handle it then tbh, as a proxy on the same machine
- you run this as a proxy, on the same machine for other reasons
- you somehow came up with an even more questionable use for this software

## how to setup
in the config file (config.json),
- smtp-data-limit is the amount of bytes a single smtp message may contain.
- smtp-timeout is the smtp timeout in seconds.
- smtp-max-recipients is the maximum allowed amount of recipients of an email forwarded.
I think it's pretty obvious.
routes is an array of routes (roaring applause from the audience reading this)
where each route defines a server listening to "port-in", and forwards the traffic to "destination" at "port-out", using either type "smtp" or "tcp".
again, incomming traffic is plain, outgoing traffic is tls encrypted.


## disclaimer
this is highly questionable MacGuffin I wrote as a fun little go exercise a while ago and decided to put up here. so if you *need* it, that means you're running insecure or outdated (and therefore insecure) software but if you feel like switching to something else would be a huge hussle, feel free to use this software provided as is :) and have fun
