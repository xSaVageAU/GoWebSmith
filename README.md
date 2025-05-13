# GoWebSmith Project

**GoWebSmith** empowers you to build powerful and maintainable web applications in Go. It's designed around a **modulithic architecture**, offering the deployment simplicity of a single unit while fostering strong internal modularity. This approach, combined with modern server-driven UI patterns (inspired by principles like "Hypermedia as the Engine of Application State"), allows for rich user experiences with streamlined development.

At its core, GoWebSmith is evolving to help you structure your website by composing **Pages** from reusable **Modules (Components)**. You build a library of independent UI components (Modules), each with its own templates and styling. Then, you define Pages that arrange these Modules within a chosen layout, creating cohesive and interactive web experiences.

*(The GIF below demonstrates the current workflow, which treats each page as a manageable "module." This will evolve as we fully implement the Page/Module component architecture.)*

See this page-centric, modulithic workflow in action! This GIF demonstrates how GoWebSmith brings this approach to life:
*   Rapidly creating a new page (currently termed a module).
*   Editing its content templates, showcasing the live preview capabilities.
*   Adding new templates to enrich the page's structure or style.
*   Viewing the fully assembled, server-rendered page-module as a user would.
*   And finally, managing the module's lifecycle, including deletion.

![GoWebSmith-gif](https://github.com/user-attachments/assets/24eee301-85d4-4a7b-aa96-37d0d876df07)

This robust system is made possible by GoWebSmith's integrated components:

*   An **intuitive Admin UI** for managing your library of Modules (components), editing their templates with live previews, and (in the future) visually composing Pages.
*   A **command-line interface (CLI)** for efficient scaffolding of Modules, managing their lifecycle, and (in the future) defining Page structures.
*   A **dynamic web server** that discovers your Pages and Modules, assembles them on demand, and serves the content to users.

## Core Vision: Pages Composing Modules

GoWebSmith is evolving towards a clear distinction between:

1.  **Modules (Components):** Reusable, self-contained UI building blocks (e.g., a navigation bar, product card, hero banner). Each Module has its own templates (HTML, CSS, etc.) and can be configured.
2.  **Pages:** The actual web pages users visit, defined by a URL slug. Each Page specifies a layout and an arrangement of Module instances, each potentially with unique configurations.

This approach promotes:
*   **Reusability:** Use the same Module in multiple places across different Pages.
*   **Maintainability:** Update a Module once, and the changes reflect everywhere it's used.
*   **Scalability:** Easily manage complex sites by breaking them into manageable components and compositions.

## Core Components

1.  **Admin UI (`cmd/admin/`)**: A web-based interface on `http://localhost:8081`.
    *   Dashboard to view and manage your library of UI Modules (components).
    *   Forms for creating new Modules.
    *   Integrated CodeMirror editor for Module template files (HTML, CSS, JS, TMPL) with live preview.
    *   Functionality to add/remove template files within Modules.
    *   Options for soft-deleting (archiving) and force-deleting Modules.
    *   *(Future: Will include tools for creating and editing Pages, arranging Modules within them.)*

2.  **Builder CLI (`cmd/builder-cli/`)**: A command-line tool for managing Modules.
    *   Module creation with standardized directory structure (`modules/{moduleID}/templates/`) and boilerplate files.
    *   Automatic generation/management of Module metadata (JSON in `.module_metadata/`).
    *   Adding/updating Module metadata (name, group, description).
    *   Soft and hard deletion of Modules.
    *   *(Future: Will include commands for Page creation, and managing Module placement on Pages.)*

3.  **Main Web Server (`cmd/server/`)**: The dynamic Go web server.
    *   Runs on `https://localhost:8443` by default (configurable).
    *   Discovers Modules (and future Pages) via metadata.
    *   Loads and parses Module templates.
    *   *(Current: Serves "page-modules" based on their URL slugs.)*
    *   *(Future: Will serve Pages by composing them from multiple Module instances based on Page metadata.)*
    *   Supports HTMX for dynamic client-side interactions.
    *   Serves global and Module-specific static assets.

## Features

*   **Modulithic Architecture**: Simplicity of a single deployable unit with strong internal modularity.
*   **Component-Based Design**: Build websites by combining independent, reusable Modules (components). *(Evolving feature)*
*   **CLI for Developers**: Efficiently manage Module lifecycle.
*   **Visual Admin UI**: Intuitive web interface for Module management and template editing.
*   **Dynamic Rendering**: Server intelligently assembles content.
*   **Templating**: Uses Go's `html/template` package.
*   **Configuration**: Server behavior managed via `config.yaml`.
*   **HTMX Integration**: Enables modern, dynamic user experiences.
*   **Static Asset Serving**: Handles global and Module-specific static files.
*   **Metadata Driven**: Behavior and rendering controlled by JSON metadata.

## Project Structure

*   `cmd/admin/`: Source code for the Admin UI.
*   `cmd/builder-cli/`: Source code for the Builder CLI tool.
*   `cmd/server/`: Source code for the Main Web Server.
*   `internal/`: Shared packages:
    *   `generator/`: Module boilerplate generation.
    *   `model/`: Data structures (`Module`, `Template`, *Future: `Page`*).
    *   `modulemanager/`: Core logic for module management operations.
    *   `storage/`: Metadata persistence (JSON-based, *Future: Page metadata store*).
    *   `templating/`: Server-side template processing.
*   `modules/`: Root directory for active Module (component) files. Each resides in a subdirectory named by its ID.
*   `modules_removed/`: Directory for soft-deleted Module files.
*   `.module_metadata/`: Stores JSON metadata files for each Module (component).
*   *Future: `.page_metadata/`: Will store JSON metadata for Pages.*
*   `web/`:
    *   `admin/`: Static assets (CSS, JS) and HTML templates for the Admin UI.
    *   `static/`: Global static assets for the Main Web Server.
    *   `templates/`: Global layout templates for the Main Web Server.
*   `pkg/fsutils/`: Filesystem utility functions.
*   `config.yaml`: Configuration file for the Main Web Server.
*   `cert.pem`, `key.pem`: TLS certificate and key files (auto-generated if not present).

## Prerequisites

*   Go (version 1.24.2 or later, as specified in `go.mod`)

## Building the Project

1.  **Build the Builder CLI**:
    ```bash
    # For Windows
    go build -o builder-cli.exe ./cmd/builder-cli
    # For Linux/macOS
    go build -o builder-cli ./cmd/builder-cli
    ```

2.  **Build the Admin UI**:
    ```bash
    # For Windows
    go build -o admin.exe ./cmd/admin
    # For Linux/macOS
    go build -o admin ./cmd/admin
    ```

3.  **Build the Main Web Server**:
    ```bash
    # For Windows
    go build -o server.exe ./cmd/server
    # For Linux/macOS
    go build -o server ./cmd/server
    ```

## Running the Project

### 1. Running the Admin UI

*   Execute the built Admin UI application:
    ```bash
    # For Windows
    .\admin.exe
    # For Linux/macOS
    ./admin
    ```
*   The Admin UI runs on port `8081` by default (configurable in `config.yaml`).
*   Access: `http://localhost:{ADMIN_PORT}` (e.g., `http://localhost:8081`)

### 2. Running the Main Web Server

*   Execute the built Main Web Server application:
    ```bash
    # For Windows
    .\server.exe
    # For Linux/macOS
    ./server
    ```
*   The server starts on `https://localhost:8443` by default.
*   Self-signed certificates (`cert.pem`, `key.pem`) are auto-generated if missing (expect browser warnings).
*   Configuration: `config.yaml`.
*   Optional module list page: `.\server.exe -toggle-module-list`

### 3. Using the Builder CLI

The Builder CLI (`builder-cli.exe` or `./builder-cli`) provides commands for managing "Modules" (which will evolve to mean "components"). Page management commands will be added in the future.

*   **`list`**: Lists all known modules (currently page-like modules).
    ```bash
    .\builder-cli list
    ```

*   **`create`**: Creates a new module.
    ```bash
    .\builder-cli create -name "My New Module" [-slug "my-custom-slug"]
    ```
    *   `-name`: (Required) The user-friendly name.
    *   `-slug`: (Optional) A custom URL-friendly slug (relevant for current page-module behavior).

*   **`update`**: Updates module metadata.
    ```bash
    .\builder-cli update -id <module-id> [-name <new-name>] [-slug <new-slug>] [-group <group>] [-layout <layout>] [-desc <description>]
    ```

*   **`delete`**: Deletes modules.
    ```bash
    # Soft delete
    .\builder-cli delete -id <module-id>

    # Hard delete
    .\builder-cli delete -id <module-id> --force

    # DANGER: Nuke all
    .\builder-cli delete --nuke-all
    ```

*   **`add-template`**: Adds a new template file to a module.
    ```bash
    .\builder-cli add-template -moduleId <module-id> -name <template-filename.ext>
    ```

*   **`preview`**: Generates a preview of a module.
    ```bash
    .\builder-cli preview -id <module-id>
    ```

*   **`purge-removed`**: Permanently deletes all soft-deleted modules.
    ```bash
    .\builder-cli purge-removed
    ```

## Configuration

The project uses a `config.yaml` file in the project root:

```yaml
# Main Web Server Configuration
server:
  port: "8443"      # Port for the HTTPS Main Web Server
  certFile: "cert.pem" # Path to the TLS certificate file for the Main Web Server
  keyFile: "key.pem"   # Path to the TLS key file for the Main Web Server

# Admin Server Configuration
admin_server:
  port: "8081"      # Port for the Admin UI HTTP server

# Add other configuration sections as needed
```
## How It Works (Current & Evolving)

Currently, GoWebSmith primarily treats each URL-addressable part of your site as a self-contained "Module." This "Module" has its own `base.html` and other templates, which are assembled and served when its URL is requested.

**The architecture is evolving towards a more granular component-based system:**

1.  **Module (Component) Creation (CLI/Admin UI):**
    *   A unique ID is generated for a reusable UI component (e.g., "Product Card," "Navigation Menu").
    *   A directory `modules/{moduleID}/templates/` is created containing templates specific to that component (e.g., `card_content.html`, `card_styles.css`). It will not contain a full page `base.html`.
    *   A JSON metadata file (`.module_metadata/{moduleID}.json`) stores the component's definition (name, description, its template files, configuration options it accepts).

2.  **Page Creation & Composition (Future - CLI/Admin UI):**
    *   A new "Page" entity will be defined (e.g., "Homepage," "Product Listing") with its own metadata, likely in a `.page_metadata/` directory.
    *   Page metadata will specify:
        *   A unique Page ID, Name, and URL Slug.
        *   A global layout template to use (e.g., from `web/templates/default_layout.html`).
        *   A list of **Module Instances**. Each instance defines:
            *   Which reusable Module (component) ID to use.
            *   Which placeholder in the global layout it should be rendered into (e.g., "header," "sidebar," "main_content").
            *   An order for rendering if multiple Modules share a placeholder.
            *   Instance-specific configuration data to be passed to the Module during rendering.

3.  **Template Editing (Admin UI):**
    *   Users will select a Module (component) and edit its specific template files.
    *   The live preview will show the component, potentially within a configurable test page context or in isolation.

4.  **Server Rendering (Main Web Server - Evolved):**
    *   On startup, the server will scan for Page and Module (component) metadata.
    *   When a request comes for a Page URL (e.g., `/homepage`):
        *   The server loads the Page's metadata.
        *   It identifies the global layout template and the list of Module instances to render.
        *   For each Module instance:
            *   It loads the specified reusable Module (component).
            *   It renders the Module's primary template(s), passing any instance-specific configuration from the Page's metadata.
        *   The rendered HTML output for each Module instance is then injected into its designated placeholder within the global layout template.
        *   The fully assembled Page HTML is sent to the client.
    *   HTMX will continue to be leveraged for dynamic interactions, potentially updating individual Module instances or sections of a Page.

This refined approach will allow for greater flexibility and reusability, enabling the construction of complex web applications from a well-defined library of components, composed into distinct Pages.

## Future Improvements & Roadmap

GoWebSmith is an evolving project. Here are some areas we're actively exploring and planning:

*   **Full Page & Module (Component) Architecture:**
    *   Complete the transition to distinct "Pages" (URL-addressable views) and "Modules" (reusable UI components).
    *   Implement a robust "Page Management" system for composing Pages from a library of shared Modules.
    *   Allow Module instances on a Page to have unique configurations.
*   **Advanced Admin UI for Page Composition:**
    *   Develop tools within the Admin UI for visually or declaratively assembling Pages by selecting, arranging, and configuring Modules within layout placeholders.
*   **Theme Engine/Global Styling:**
    *   Mechanisms for global theme application and easier customization of site-wide aesthetics.
*   **Data Source Integration:**
    *   Flexible ways for Modules to fetch and display dynamic data from various sources.
*   **User Authentication and Authorization:**
    *   Integrating access control for the Admin UI and potentially for different site sections.
*   **Plugin System:**
    *   Exploring a plugin architecture to extend GoWebSmith's core functionality.
*   **Internationalization (i18n) and Localization (l10n).**
*   **Continued Performance Optimizations and Deployment Strategies.**

We welcome community feedback and contributions as we work towards these goals!
