import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { ActionInputForm } from "./ActionInputForm";

// What an automation's action asks before it runs: words, a number, a
// yes, one of its words.

const meta: Meta<typeof ActionInputForm> = { title: "Automations/ActionInputForm", component: ActionInputForm };
export default meta;

export const EveryKind: StoryObj<typeof ActionInputForm> = {
  render: () => {
    const [values, setValues] = useState<Record<string, unknown>>({ note: "It's been a week", how: "comment", loud: false, days: "3" });
    return (
      <div style={{ width: 380 }}>
        <ActionInputForm
          fields={[
            { name: "note", type: "text" },
            { name: "days", type: "int" },
            { name: "how", type: "choice", choices: ["comment", "ping"] },
            { name: "loud", type: "bool" },
          ]}
          values={values}
          onChange={(name, v) => setValues((was) => ({ ...was, [name]: v }))}
          onGo={() => {}}
        />
      </div>
    );
  },
};
