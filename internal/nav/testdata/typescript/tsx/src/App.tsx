import { Button } from "./Button";

export function App() {
  return (
    <div>
      <Button label="go" />
      //^ resolved src/Button.tsx:Button "imported from ./Button"
    </div>
  );
}
