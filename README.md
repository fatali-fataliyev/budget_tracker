# Budget Tracker App 💰

A RESTful API built with Go 🦫 for managing personal budgets, expenses, and income categories.

---

## 📦 Requirements

- Go 1.20 or later
- MySQL database
- Internet connection
- A server (for production)

---

## ⚙️ Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/yourusername/budget-tracker.git
   cd budget-tracker
   ```

2. **Setup environment variables**
   ```bash
   cp env_sample .env
   ```
   Then open the **.env** file and fill in the required values such as your dbuser, dbpassword, dbname etc.
3. **Run the application**
   ```bash
   go run main.go
   ```
   **Note: You can define the port in the .env file. If you don't, port 8060 will be used by default.**

---

## 📑 View API Documentation

[Click here to view the full OpenAPI documentation](https://editor.swagger.io/?url=https://raw.githubusercontent.com/fatali-fataliyev/budget_tracker/main/openapi.yaml)

---

## Features

- **User registration, login, and logout**

- **Create, read, update, delete (CRUD) for both categories**

- **Expense and income categories**

- **OCR support: upload images of receipts to auto-fill transaction fields**

- **CORS-ready for frontend integration**
