# 📅 Booking System Demo

Hey! This is a simple demo showing how you can use **HorneroDB** to power a real-world business, like a hair salon or a dentist's office. It has two parts:
1.  **Public Site**: Where customers see your prices and book a spot.
2.  **Admin Panel**: Where the owner logs in to manage everything.

---

## 🛠 Step 1: Set up HorneroDB

First, you need to create the "brain" for your app in the HorneroDB dashboard.

### 1. Create a Workspace
Go to your dashboard and create a new workspace (e.g., "My Salon"). **Copy the Workspace ID** from the URL or settings; you'll need it later.

### 2. Create the "servicios" Table
Create a table called `servicios` with these columns:
*   `nombre` (Text)
*   `descripcion` (Long Text)
*   `precio` (Number)

Add a few rows like "Haircut - $25" so you have something to see!

### 3. Create the "turnos" Table
Create a table called `turnos` with these columns:
*   `from` (DateTime)
*   `to` (DateTime)
*   `client_name` (Text)
*   `client_email` (Email)
*   `client_phone` (Text)
*   `servicio_id` (Text)

### 4. Create a Public API Key
Go to **Settings > API Keys** and create a new key called "Public Website".
*   **Permissions**: 
    *   `servicios`: Give it "Read" access to all columns.
    *   `turnos`: 
        *   **Read**: ONLY check the `from` and `to` columns. This keeps your customers' private info safe!
        *   **Create**: Check all columns (so people can actually book).
*   **Security**: Set the "Allowed Origins" to wherever you are running this (like `http://localhost:5500`).

---

## ⚙️ Step 2: Configure the Code

You need to tell the websites how to talk to your database.

1.  **Public Site**: Open `public/config.js` and paste your **Workspace ID** and **API Key**.
2.  **Admin Site**: Open `admin/config.js` and paste your **Workspace ID**. (The admin site uses your login, so it doesn't need an API key).

---

## 🚀 Step 3: Run it!

Since these are just plain HTML/CSS/JS files, you can use any simple web server.

If you have VS Code, just right-click `index.html` and choose **"Open with Live Server"**.

*   **Public**: `http://localhost:5500/public/index.html`
*   **Admin**: `http://localhost:5500/admin/index.html`

---

## 🔐 How the security works (The "Magic" part)

*   **Privacy**: When the public site asks for `turnos`, HorneroDB sees the API key permissions and automatically **removes** the names and phone numbers from the list. The website only receives the times, so nobody can "inspect element" to steal data.
*   **Admin Access**: The Admin panel uses **PocketID**. Only people you manually add to your Workspace in HorneroDB can log in to see the full client list or change prices.
