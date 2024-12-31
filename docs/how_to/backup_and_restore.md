# Backup and Restore

This how-to guide provides step-by-step instructions for backing up and restoring the Ella Core database. Here, we will create a backup of the Ella Core database, download the backup file, and restore the database from the backup file on a new instance of Ella Core.

Pre-requisites:

- Access to the Ella Core API
- Curl

=== "Backup"

    1. Get a token by logging in to Ella Core.

    ```shell
    curl -k -X POST https://127.0.0.1:5001/api/v1/login --data '{"username":"<your username>","password":"<your password>"}'
    ```

    2. Use the token to create a backup of the Ella Core database. Here we output the backup file to `backup_file.db`.

    ```shell
    curl -k -X POST https://127.0.0.1:5001/api/v1/backup   -H "Authorization: Bearer <Your Token>"   -o backup_file.db
    ```

=== "Restore"

    Run the following commands on a new instance of Ella Core.
    
    1. Get a token by logging in to Ella Core.

    ```shell
    curl -k -X POST https://127.0.0.1:5001/api/v1/login --data '{"username":"<your username>","password":"<your password>"}'
    ```

    2. Use the token to restore the Ella Core database from a backup file. Here we restore the database from `backup_file.db`.

    ```shell
    curl -k -X POST https://127.0.0.1:5001/api/v1/restore -H "Authorization: Bearer <Your Token>" -F "backup=@backup_file.db"
    ```
