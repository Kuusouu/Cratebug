import { useEffect, useState } from "react";
import { RuntimeStatus } from "../wailsjs/go/main/App";
import "./App.css";

type Connection = {
  state: "checking" | "connected" | "error";
  message: string;
};

function App() {
  const [connection, setConnection] = useState<Connection>({
    state: "checking",
    message: "Connecting to Go backend",
  });

  useEffect(() => {
    let active = true;

    RuntimeStatus()
      .then((message) => {
        if (active) {
          setConnection({ state: "connected", message });
        }
      })
      .catch(() => {
        if (active) {
          setConnection({
            state: "error",
            message: "Go backend unavailable",
          });
        }
      });

    return () => {
      active = false;
    };
  }, []);

  return (
    <main className="shell">
      <header className="brand">
        <div className="brand-mark" aria-hidden="true">
          C
        </div>
        <div>
          <p className="brand-kicker">Marvel Rivals mod manager</p>
          <h1>Cratebug</h1>
        </div>
      </header>

      <section className="foundation-card">
        <div className="foundation-copy">
          <p className="eyebrow">Foundation preview</p>
          <h2>Ready for the library.</h2>
          <p>
            The application shell is running. Mod discovery and management
            arrive in later phases.
          </p>
        </div>

        <div className="connection" data-state={connection.state}>
          <span className="connection-dot" aria-hidden="true" />
          <span>{connection.message}</span>
        </div>
      </section>

      <footer>Phase 0 · Local-first Windows application</footer>
    </main>
  );
}

export default App;
