# Budget Tracker App 💰

A RESTful API built with Go 🦫 for managing personal budgets, expenses, and income categories, extract data from receipt.

---

## 📦 Requirements

- Go 1.20 or later
- MySQL database
- Internet connection(for OCR_API)

---

## ⚙️ Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/fatali-fataliyev/budget_tracker
   cd budget-tracker
   ```

2. **Setup environment variables**
   ```bash
   cp env_sample .env
   ```
   Then open the **.env** file and fill in the required values such as your _db user_, _db host_, _db password_, _dbname_ etc.
3. **Run the application**
   ```bash
   go run main.go
   ```
   **Note: You can define the port in the .env file. If you don't, port 8060 will be used by default.**

---

## 📑 View API Documentation

1. **Copy the yaml file below**

<details>
  <summary><strong>Click to expand and copy the OpenAPI YAML</strong></summary>

```yaml

```

</details>

2. **Open the [link](https://editor.swagger.io), paste the editor.**
