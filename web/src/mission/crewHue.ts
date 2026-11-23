// crewHue is a crew's own colour within its mission, the same wherever it
// shows: its badge, its bar in the timeline.
const hues = [228, 152, 32, 330, 192, 272, 96, 8];
export function crewHue(id: string): number {
  const n = Number(/\d+/.exec(id)?.[0] ?? 0);
  return hues[(Math.max(n, 1) - 1) % hues.length];
}
