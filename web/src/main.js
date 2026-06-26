import { mount } from "svelte";
import App from "./App.svelte";
import "./app.css";

document.documentElement.classList.add("dark", "slate", "blue");
document.documentElement.style.colorScheme = "dark";

const app = mount(App, {
  target: document.getElementById("app"),
});

export default app;
