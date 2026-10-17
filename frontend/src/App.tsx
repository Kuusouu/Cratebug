// Load global tokens and primitives before LibraryScreen so component
// CSS modules can override equal-specificity rules such as dialog width.
import "./App.css";
import { TooltipProvider } from "./components/ui/tooltip";
import { LibraryScreen } from "./library/LibraryScreen";

function App() {
	return (
		<TooltipProvider>
			<LibraryScreen />
		</TooltipProvider>
	);
}

export default App;
