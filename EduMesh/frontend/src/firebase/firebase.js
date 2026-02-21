// firebase.js
import { initializeApp } from "firebase/app";
import { getAuth, GoogleAuthProvider } from "firebase/auth";

const firebaseConfig = {
  apiKey: "AIzaSyDVYQWT6bc8bjJsnFITyxncKbho3TvANts",
  authDomain: "prabal-rural-empower.firebaseapp.com",
  projectId: "prabal-rural-empower",
  storageBucket: "prabal-rural-empower.firebasestorage.app",
  messagingSenderId: "590807847457",
  appId: "1:590807847457:web:d6b7325cd412330da78960",
};

const app = initializeApp(firebaseConfig);
const auth = getAuth(app);
const provider = new GoogleAuthProvider();

export { auth, provider };
