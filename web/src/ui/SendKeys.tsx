import { Keys } from "./Keys";
import { sendKeys, useSendKey } from "./sendKey";

// SendKeys is the keys that send what's written in a box, as you've
// chosen them (ui/sendKey.ts): what a button that sends says beside its
// name. other is the box's second way of sending.
export function SendKeys({ other, className }: { other?: boolean; className?: string }) {
  return <Keys k={sendKeys(useSendKey(), other)} className={className} />;
}
