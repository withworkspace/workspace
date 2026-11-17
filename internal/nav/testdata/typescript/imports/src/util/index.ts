export function shout(s: string) {
  return s + "!";
}
const quiet = (s: string) => s;
export { quiet as whisper };
export default function greet() {
  return "hi";
}
