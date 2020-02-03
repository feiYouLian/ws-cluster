
As I mentioned here, the WebSockets protocol is, at this point, a bit of a mess due to the evolution of the protocol and the fact that it's being pulled in various directions by various interested parties. I'm just ranting about some of the things that I find annoying...

   The client MUST mask all frames sent to the server.  A server MUST
   close the connection upon receiving a frame with the MASK bit set to
   0.  In this case, a server MAY send a close frame with a status code
   of 1002 (protocol error) as defined in Section 7.4.1.
Fair enough, but why?
The masking in itself is relatively painless to achieve but it interacts poorly with the albeit badly designed deflate-stream extension and forcing the client to mask zero length frames seems unnecessary - though I agree that special cases don't really seem warranted.

The RFC doesn't explain why masking from client to server is considered essential but a search through the discussion list brings up plenty of hits. The best descriptions of why can be found here and here.

Masking of WebSocket traffic from client to server is required because of the unlikely chance that malicious code could cause some broken proxies to do the wrong thing and use this as an attack of some kind. Nobody has proved that this could actually happen, but since the fact that it could happen was reason enough for browser vendors to get twitchy, masking was added to remove the possibility of it being used as an attack.

The idea being that since the API level code generating the WebSocket frame gets to select a masking key and mask the data supplied by the application code the application code cannot in any meaningful way dictate the data that ends up passing through the potentially broken intermediaries and therefore can't cause trouble. Since the masking key is in the frame intermediaries can be written to understand and unmask the data to perform some form of clever inspection if they want to.

Now, would it be so hard to add something that explains this to the RFC?
