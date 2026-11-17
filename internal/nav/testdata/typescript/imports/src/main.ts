import greet, { shout, whisper } from "@util";
import { ANON, type User } from "@models/user";
import * as users from "./models/user.js";
import { Button, Panel } from "@acme/ui";
import { Button as B2 } from "@acme/ui/button";
import { readFileSync } from "node:fs";
import React from "react";

const u: User = ANON;
//       ^ resolved src/models/user.ts:User "imported from @models/user"
//              ^ resolved src/models/user.ts:ANON "imported from @models/user"
shout(greet());
// <- resolved src/util/index.ts:shout "imported from @util"
//    ^ resolved src/util/index.ts:greet "imported from @util"
whisper(users.ANON.name);
// <- resolved src/util/index.ts:quiet "imported from @util"
//            ^ resolved src/models/user.ts:ANON "imported from ./models/user.js"
//                 ^ resolved src/models/user.ts:name
Button(u.name);
// <- resolved packages/ui/src/button.ts:Button "imported from @acme/ui"
new Panel();
//  ^ resolved packages/ui/src/card.ts:Card "imported from @acme/ui"
B2("x");
// <- resolved packages/ui/src/button.ts:Button "imported from @acme/ui/button"
readFileSync("x");
// <- external node:fs
React.createElement("div");
// <- external react
//    ^ external react
